#!/usr/bin/env python3
"""Example worker application for PubSub Scheduler"""

import logging
import os
import signal
import sys
from datetime import timedelta

from google.cloud import pubsub_v1

from pubsubscheduler import Scheduler, WorkerOptions

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def main():
    # Get configuration from environment
    database_url = os.getenv("DATABASE_URL")
    if not database_url:
        logger.error("DATABASE_URL environment variable is required")
        sys.exit(1)

    project_id = os.getenv("PUBSUB_PROJECT_ID")
    if not project_id:
        logger.error("PUBSUB_PROJECT_ID environment variable is required")
        sys.exit(1)

    worker_id = os.getenv("WORKER_ID") or os.getenv("HOSTNAME")

    # Create Pub/Sub client
    pubsub_client = pubsub_v1.PublisherClient()
    logger.info("Connected to Pub/Sub")

    # Create scheduler
    scheduler = Scheduler(database_url, pubsub_client, worker_id)
    logger.info(f"Initialized scheduler with worker_id={scheduler.worker_id}")

    # Set up graceful shutdown
    shutdown_requested = False

    def signal_handler(signum, frame):
        nonlocal shutdown_requested
        logger.info(f"Received signal {signum}, shutting down...")
        shutdown_requested = True

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    try:
        # Run worker (blocking)
        logger.info("Starting worker...")
        opts = WorkerOptions(
            batch_size=10,
            lease_duration=timedelta(minutes=5),
            poll_interval=timedelta(seconds=1)
        )
        scheduler.run_worker(opts)
    except KeyboardInterrupt:
        logger.info("Shutdown complete")
    finally:
        scheduler.close()


if __name__ == "__main__":
    main()
