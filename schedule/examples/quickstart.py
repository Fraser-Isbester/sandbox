#!/usr/bin/env python3
"""
Quick Start Example - Schedule and publish messages to Pub/Sub

This example demonstrates:
1. Scheduling one-time messages
2. Bulk scheduling many messages
3. Creating recurring schedules
4. Running a worker to publish messages
"""

import os
import sys
from datetime import datetime, timedelta

from google.cloud import pubsub_v1

# Add parent directory to path to import pubsubscheduler
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'python'))

from pubsubscheduler import Scheduler, ScheduleSpec, WorkerOptions


def main():
    # Configuration
    database_url = os.getenv("DATABASE_URL", "postgresql://localhost/scheduler")
    project_id = os.getenv("PUBSUB_PROJECT_ID", "your-project-id")

    # Create clients
    pubsub_client = pubsub_v1.PublisherClient()
    scheduler = Scheduler(database_url, pubsub_client, "quickstart-example")

    print("PubSub Scheduler Quick Start\n")

    # Example 1: Schedule a single message
    print("1. Scheduling a single message for 5 seconds from now...")
    message_id = scheduler.schedule_message(
        topic="test-topic",
        message={"type": "test", "value": "hello world"},
        scheduled_at=datetime.now() + timedelta(seconds=5)
    )
    print(f"   ✓ Scheduled message: {message_id}\n")

    # Example 2: Bulk schedule multiple messages
    print("2. Bulk scheduling 100 messages for 10 seconds from now...")
    messages = [
        {"type": "bulk_test", "index": i, "value": f"message-{i}"}
        for i in range(100)
    ]
    scheduler.bulk_schedule_messages(
        topic="test-topic",
        messages=messages,
        scheduled_at=datetime.now() + timedelta(seconds=10)
    )
    print(f"   ✓ Scheduled {len(messages)} messages\n")

    # Example 3: Create a recurring schedule
    print("3. Creating a recurring schedule (every minute)...")
    schedule_id = scheduler.create_schedule(ScheduleSpec(
        name="minutely-test",
        cron_expression="* * * * *",  # Every minute
        topic="test-topic",
        message={"type": "recurring", "source": "cron"}
    ))
    print(f"   ✓ Created schedule: {schedule_id}\n")

    # Example 4: List all schedules
    print("4. Listing all schedules...")
    schedules = scheduler.list_schedules()
    for schedule in schedules:
        print(f"   - {schedule.name}: {schedule.cron_expression}")
        print(f"     Next run: {schedule.next_run_at}")
    print()

    # Example 5: Query scheduled messages
    print("5. Querying database for scheduled messages...")
    import psycopg2
    conn = psycopg2.connect(database_url)
    with conn.cursor() as cur:
        cur.execute("SELECT status, COUNT(*) FROM scheduled_messages GROUP BY status")
        results = cur.fetchall()
        print("   Status distribution:")
        for status, count in results:
            print(f"   - {status}: {count}")
    conn.close()
    print()

    # Example 6: Run worker (uncomment to actually publish messages)
    print("6. To run a worker and publish messages, uncomment the code below:")
    print("   scheduler.run_worker(WorkerOptions())")
    print()
    print("   Or run in a separate terminal:")
    print(f"   DATABASE_URL={database_url} PUBSUB_PROJECT_ID={project_id} python python/examples/main.py")
    print()

    # Clean up: disable the recurring schedule
    print("Cleaning up: disabling recurring schedule...")
    scheduler.disable_schedule(schedule_id)
    print("✓ Done!\n")

    scheduler.close()


if __name__ == "__main__":
    main()
