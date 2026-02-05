"""Core scheduler implementation for PubSub Scheduler"""

import json
import logging
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional

import psycopg2
import psycopg2.extras
from croniter import croniter
from google.cloud import pubsub_v1

logger = logging.getLogger(__name__)


@dataclass
class ScheduleSpec:
    """Specification for a recurring schedule"""
    name: str
    cron_expression: str
    topic: str
    message: Dict[str, Any]
    enabled: bool = True
    id: Optional[str] = None
    next_run_at: Optional[datetime] = None
    last_run_at: Optional[datetime] = None


@dataclass
class WorkerOptions:
    """Configuration options for worker behavior"""
    batch_size: int = 10
    lease_duration: timedelta = timedelta(minutes=5)
    poll_interval: timedelta = timedelta(seconds=1)


class Scheduler:
    """Manages scheduling and publishing messages to Google Cloud Pub/Sub"""

    def __init__(
        self,
        db_url: str,
        pubsub_client: pubsub_v1.PublisherClient,
        worker_id: Optional[str] = None
    ):
        """
        Initialize scheduler with database and Pub/Sub client

        Args:
            db_url: PostgreSQL connection string
            pubsub_client: Google Cloud Pub/Sub publisher client
            worker_id: Unique identifier for this worker (defaults to random UUID)
        """
        self.db_url = db_url
        self.pubsub_client = pubsub_client
        self.worker_id = worker_id or str(uuid.uuid4())
        self._conn: Optional[psycopg2.extensions.connection] = None

    def _get_connection(self) -> psycopg2.extensions.connection:
        """Get or create database connection"""
        if self._conn is None or self._conn.closed:
            self._conn = psycopg2.connect(self.db_url)
        return self._conn

    def close(self):
        """Close database connection"""
        if self._conn and not self._conn.closed:
            self._conn.close()

    def schedule_message(
        self,
        topic: str,
        message: Dict[str, Any],
        scheduled_at: datetime
    ) -> str:
        """
        Schedule a single Pub/Sub message

        Args:
            topic: Pub/Sub topic name
            message: Message payload as dictionary
            scheduled_at: When to publish the message

        Returns:
            Message ID (UUID)
        """
        message_id = str(uuid.uuid4())
        message_json = json.dumps(message)

        conn = self._get_connection()
        with conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO scheduled_messages (id, topic, message, scheduled_at, status)
                VALUES (%s, %s, %s, %s, 'pending')
                """,
                (message_id, topic, message_json, scheduled_at)
            )
        conn.commit()

        return message_id

    def bulk_schedule_messages(
        self,
        topic: str,
        messages: List[Dict[str, Any]],
        scheduled_at: datetime
    ) -> None:
        """
        Bulk schedule messages (for 10M+ messages)

        Args:
            topic: Pub/Sub topic name
            messages: List of message payloads
            scheduled_at: When to publish the messages
        """
        if not messages:
            return

        conn = self._get_connection()
        with conn.cursor() as cur:
            # Use execute_batch for efficient bulk insert
            psycopg2.extras.execute_batch(
                cur,
                """
                INSERT INTO scheduled_messages (id, topic, message, scheduled_at, status)
                VALUES (%s, %s, %s, %s, 'pending')
                """,
                [
                    (str(uuid.uuid4()), topic, json.dumps(msg), scheduled_at)
                    for msg in messages
                ],
                page_size=1000
            )
        conn.commit()

    def create_schedule(self, spec: ScheduleSpec) -> str:
        """
        Create a recurring schedule

        Args:
            spec: Schedule specification

        Returns:
            Schedule ID (UUID)
        """
        # Validate cron expression
        if not croniter.is_valid(spec.cron_expression):
            raise ValueError(f"Invalid cron expression: {spec.cron_expression}")

        # Calculate next run time
        cron = croniter(spec.cron_expression, datetime.now())
        next_run = cron.get_next(datetime)

        schedule_id = str(uuid.uuid4())
        message_json = json.dumps(spec.message)

        conn = self._get_connection()
        with conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO schedules (id, name, cron_expression, topic, message, enabled, next_run_at)
                VALUES (%s, %s, %s, %s, %s, %s, %s)
                """,
                (schedule_id, spec.name, spec.cron_expression, spec.topic, message_json, spec.enabled, next_run)
            )
        conn.commit()

        return schedule_id

    def update_schedule(self, schedule_id: str, spec: ScheduleSpec) -> None:
        """
        Update an existing schedule

        Args:
            schedule_id: Schedule ID to update
            spec: New schedule specification
        """
        # Validate cron expression
        if not croniter.is_valid(spec.cron_expression):
            raise ValueError(f"Invalid cron expression: {spec.cron_expression}")

        # Calculate next run time
        cron = croniter(spec.cron_expression, datetime.now())
        next_run = cron.get_next(datetime)

        message_json = json.dumps(spec.message)

        conn = self._get_connection()
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE schedules
                SET name = %s, cron_expression = %s, topic = %s, message = %s,
                    enabled = %s, next_run_at = %s, updated_at = NOW()
                WHERE id = %s
                """,
                (spec.name, spec.cron_expression, spec.topic, message_json, spec.enabled, next_run, schedule_id)
            )
            if cur.rowcount == 0:
                raise ValueError(f"Schedule not found: {schedule_id}")
        conn.commit()

    def disable_schedule(self, schedule_id: str) -> None:
        """
        Disable a recurring schedule

        Args:
            schedule_id: Schedule ID to disable
        """
        conn = self._get_connection()
        with conn.cursor() as cur:
            cur.execute(
                "UPDATE schedules SET enabled = false, updated_at = NOW() WHERE id = %s",
                (schedule_id,)
            )
            if cur.rowcount == 0:
                raise ValueError(f"Schedule not found: {schedule_id}")
        conn.commit()

    def get_schedule(self, schedule_id: str) -> ScheduleSpec:
        """
        Get schedule by ID

        Args:
            schedule_id: Schedule ID to retrieve

        Returns:
            Schedule specification
        """
        conn = self._get_connection()
        with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
            cur.execute(
                """
                SELECT id, name, cron_expression, topic, message, enabled, next_run_at, last_run_at
                FROM schedules
                WHERE id = %s
                """,
                (schedule_id,)
            )
            row = cur.fetchone()
            if not row:
                raise ValueError(f"Schedule not found: {schedule_id}")

            return ScheduleSpec(
                id=row['id'],
                name=row['name'],
                cron_expression=row['cron_expression'],
                topic=row['topic'],
                message=json.loads(row['message']),
                enabled=row['enabled'],
                next_run_at=row['next_run_at'],
                last_run_at=row['last_run_at']
            )

    def list_schedules(self) -> List[ScheduleSpec]:
        """
        List all schedules

        Returns:
            List of schedule specifications
        """
        conn = self._get_connection()
        with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
            cur.execute(
                """
                SELECT id, name, cron_expression, topic, message, enabled, next_run_at, last_run_at
                FROM schedules
                ORDER BY created_at DESC
                """
            )
            rows = cur.fetchall()

            return [
                ScheduleSpec(
                    id=row['id'],
                    name=row['name'],
                    cron_expression=row['cron_expression'],
                    topic=row['topic'],
                    message=json.loads(row['message']),
                    enabled=row['enabled'],
                    next_run_at=row['next_run_at'],
                    last_run_at=row['last_run_at']
                )
                for row in rows
            ]
