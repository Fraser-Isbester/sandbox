"""Worker implementation for publishing messages to Pub/Sub"""

import json
import logging
import time
from datetime import datetime, timedelta
from typing import Dict, List, Any

import psycopg2
import psycopg2.extras
from croniter import croniter
from google.cloud import pubsub_v1

from .scheduler import WorkerOptions

logger = logging.getLogger(__name__)


class WorkerMixin:
    """Mixin for worker functionality - to be used with Scheduler class"""

    def run_worker(self, options: WorkerOptions = WorkerOptions()) -> None:
        """
        Run worker loop (blocking) - publishes messages to Pub/Sub

        Args:
            options: Worker configuration options
        """
        logger.info(
            f"Starting worker {self.worker_id} with batch_size={options.batch_size}, "
            f"lease_duration={options.lease_duration}, poll_interval={options.poll_interval}"
        )

        try:
            while True:
                try:
                    self._process_batch(options)
                except Exception as e:
                    logger.error(f"Error processing batch: {e}", exc_info=True)

                time.sleep(options.poll_interval.total_seconds())
        except KeyboardInterrupt:
            logger.info(f"Worker {self.worker_id} shutting down")

    def _process_batch(self, options: WorkerOptions) -> None:
        """Claim and publish a batch of messages"""
        # Claim messages atomically
        messages = self._claim_messages(options.batch_size, options.lease_duration)

        if not messages:
            return

        logger.info(f"Worker {self.worker_id} claimed {len(messages)} messages")

        # Publish each message to Pub/Sub
        for msg in messages:
            try:
                self._publish_message(msg)
                logger.info(f"Successfully published message {msg['id']} to topic {msg['topic']}")
                self._mark_message_published(msg['id'])
            except Exception as e:
                logger.error(f"Failed to publish message {msg['id']}: {e}", exc_info=True)
                self._mark_message_failed(msg['id'], str(e))

    def _claim_messages(self, limit: int, lease_duration: timedelta) -> List[Dict[str, Any]]:
        """
        Atomically claim messages ready to be published using SKIP LOCKED

        Args:
            limit: Maximum number of messages to claim
            lease_duration: How long to hold the lease

        Returns:
            List of claimed messages
        """
        conn = self._get_connection()
        with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
            # Atomic claim using SKIP LOCKED
            cur.execute(
                """
                UPDATE scheduled_messages
                SET
                    status = 'processing',
                    lease_expires_at = NOW() + %s::interval,
                    worker_id = %s,
                    started_at = NOW(),
                    attempts = attempts + 1
                WHERE id IN (
                    SELECT id FROM scheduled_messages
                    WHERE scheduled_at <= NOW()
                      AND status = 'pending'
                      AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
                    ORDER BY scheduled_at ASC
                    LIMIT %s
                    FOR UPDATE SKIP LOCKED
                )
                RETURNING id, topic, message
                """,
                (f"{int(lease_duration.total_seconds())} seconds", self.worker_id, limit)
            )
            rows = cur.fetchall()

        conn.commit()

        # Parse messages
        messages = []
        for row in rows:
            messages.append({
                'id': row['id'],
                'topic': row['topic'],
                'message': json.loads(row['message'])
            })

        return messages

    def _publish_message(self, msg: Dict[str, Any]) -> None:
        """
        Publish a message to Google Cloud Pub/Sub

        Args:
            msg: Message containing id, topic, and message payload
        """
        topic_path = self.pubsub_client.topic_path(
            self.pubsub_client.project,
            msg['topic']
        )

        # Convert message to JSON bytes
        data = json.dumps(msg['message']).encode('utf-8')

        # Publish to Pub/Sub
        future = self.pubsub_client.publish(topic_path, data)
        future.result()  # Wait for publish to complete

    def _mark_message_published(self, message_id: str) -> None:
        """Mark a message as successfully published"""
        conn = self._get_connection()
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE scheduled_messages
                SET status = 'published', published_at = NOW()
                WHERE id = %s
                """,
                (message_id,)
            )
        conn.commit()

    def _mark_message_failed(self, message_id: str, error_msg: str) -> None:
        """Mark a message as failed"""
        conn = self._get_connection()
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE scheduled_messages
                SET status = 'failed', error_message = %s
                WHERE id = %s
                """,
                (error_msg, message_id)
            )
        conn.commit()

    def run_schedule_processor(self, interval: timedelta = timedelta(seconds=10)) -> None:
        """
        Run schedule processor in background (blocking)

        Args:
            interval: How often to check for schedules to process
        """
        logger.info(f"Starting schedule processor with interval={interval}")

        try:
            while True:
                try:
                    self._process_schedules()
                except Exception as e:
                    logger.error(f"Error processing schedules: {e}", exc_info=True)

                time.sleep(interval.total_seconds())
        except KeyboardInterrupt:
            logger.info("Schedule processor shutting down")

    def _process_schedules(self) -> None:
        """Find schedules ready to run and spawn messages"""
        conn = self._get_connection()
        with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
            cur.execute(
                """
                SELECT id, name, cron_expression, topic, message, next_run_at
                FROM schedules
                WHERE enabled = true AND next_run_at <= NOW()
                ORDER BY next_run_at ASC
                """
            )
            rows = cur.fetchall()

        for row in rows:
            try:
                schedule_id = row['id']
                name = row['name']
                cron_expression = row['cron_expression']
                topic = row['topic']
                message = json.loads(row['message'])
                next_run_at = row['next_run_at']

                # Spawn message
                self._spawn_scheduled_message(schedule_id, topic, message, next_run_at)

                # Calculate next run time
                cron = croniter(cron_expression, datetime.now())
                next_run = cron.get_next(datetime)

                # Update schedule
                with conn.cursor() as update_cur:
                    update_cur.execute(
                        """
                        UPDATE schedules
                        SET last_run_at = %s, next_run_at = %s, updated_at = NOW()
                        WHERE id = %s
                        """,
                        (next_run_at, next_run, schedule_id)
                    )
                conn.commit()

                logger.info(f"Spawned message for schedule {schedule_id} ({name}), next run at {next_run}")

            except Exception as e:
                logger.error(f"Failed to process schedule {row['id']}: {e}", exc_info=True)
                conn.rollback()

    def _spawn_scheduled_message(
        self,
        schedule_id: str,
        topic: str,
        message: Dict[str, Any],
        scheduled_at: datetime
    ) -> None:
        """Create a scheduled message from a recurring schedule"""
        message_json = json.dumps(message)

        conn = self._get_connection()
        with conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO scheduled_messages (schedule_id, topic, message, scheduled_at, status)
                VALUES (%s, %s, %s, %s, 'pending')
                """,
                (schedule_id, topic, message_json, scheduled_at)
            )
        conn.commit()


# Dynamically add worker methods to Scheduler class
def _add_worker_methods():
    """Add worker methods to Scheduler class"""
    from .scheduler import Scheduler

    for attr_name in dir(WorkerMixin):
        if not attr_name.startswith('_') or attr_name in ['_process_batch', '_claim_messages',
                                                            '_publish_message', '_mark_message_published',
                                                            '_mark_message_failed', '_process_schedules',
                                                            '_spawn_scheduled_message']:
            attr = getattr(WorkerMixin, attr_name)
            if callable(attr):
                setattr(Scheduler, attr_name, attr)


_add_worker_methods()
