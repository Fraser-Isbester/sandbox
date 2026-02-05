# PubSub Scheduler - Quick Reference

## Installation

```bash
# Go
go get github.com/fraser-isbester/pubsubscheduler

# Python
pip install pubsubscheduler
```

## Database Setup

```bash
psql $DATABASE_URL < migrations/001_create_pubsub_scheduler.sql
```

## Quick Start (Python)

```python
from datetime import datetime, timedelta
from google.cloud import pubsub_v1
from pubsubscheduler import Scheduler, ScheduleSpec, WorkerOptions

# Initialize
pubsub_client = pubsub_v1.PublisherClient()
scheduler = Scheduler("postgresql://localhost/db", pubsub_client)

# Schedule one message
scheduler.schedule_message(
    topic="my-topic",
    message={"key": "value"},
    scheduled_at=datetime.now() + timedelta(hours=1)
)

# Schedule many messages
scheduler.bulk_schedule_messages(
    topic="my-topic",
    messages=[{"id": i} for i in range(1000)],
    scheduled_at=datetime.now() + timedelta(hours=1)
)

# Create recurring schedule
scheduler.create_schedule(ScheduleSpec(
    name="daily-job",
    cron_expression="0 10 * * *",  # 10am daily
    topic="my-topic",
    message={"type": "daily"}
))

# Run worker
scheduler.run_worker(WorkerOptions())
```

## Quick Start (Go)

```go
import (
    "context"
    "database/sql"
    "time"
    "cloud.google.com/go/pubsub"
    "github.com/fraser-isbester/pubsubscheduler/pubsubscheduler"
)

// Initialize
db, _ := sql.Open("postgres", "postgres://localhost/db")
pubsubClient, _ := pubsub.NewClient(ctx, "project-id")
scheduler := pubsubscheduler.NewScheduler(db, pubsubClient, "worker-1")

// Schedule one message
scheduler.ScheduleMessage(ctx, "my-topic",
    map[string]interface{}{"key": "value"},
    time.Now().Add(time.Hour))

// Schedule many messages
messages := make([]map[string]interface{}, 1000)
for i := 0; i < 1000; i++ {
    messages[i] = map[string]interface{}{"id": i}
}
scheduler.BulkScheduleMessages(ctx, "my-topic", messages, time.Now().Add(time.Hour))

// Create recurring schedule
scheduler.CreateSchedule(ctx, pubsubscheduler.ScheduleSpec{
    Name: "daily-job",
    CronExpression: "0 10 * * *",
    Topic: "my-topic",
    Message: map[string]interface{}{"type": "daily"},
})

// Run worker
opts := pubsubscheduler.DefaultWorkerOptions()
scheduler.RunWorker(ctx, opts)
```

## Monitoring Queries

```sql
-- Check message status
SELECT status, COUNT(*) FROM scheduled_messages GROUP BY status;

-- Find pending messages
SELECT COUNT(*) FROM scheduled_messages WHERE status = 'pending';

-- Active workers
SELECT worker_id, COUNT(*), MAX(started_at)
FROM scheduled_messages
WHERE status = 'processing'
GROUP BY worker_id;

-- Failed messages
SELECT id, topic, error_message, attempts
FROM scheduled_messages
WHERE status = 'failed'
ORDER BY created_at DESC
LIMIT 10;

-- Schedule status
SELECT name, enabled, next_run_at, last_run_at
FROM schedules
ORDER BY next_run_at;
```

## GKE Deployment

```bash
# Create secret
kubectl create secret generic postgres-creds \
  --from-literal=url="postgresql://user:pass@host/db"

# Deploy workers (Python)
docker build -t gcr.io/project/worker:latest -f python/examples/Dockerfile .
docker push gcr.io/project/worker:latest
kubectl apply -f python/examples/k8s/deployment.yaml

# Scale workers
kubectl scale deployment pubsub-scheduler-worker --replicas=20
```

## Common Use Cases

### Send 10M Notifications Tomorrow

```python
# Get data
patient_ids = get_all_patients()  # Returns 10M IDs

# Bulk schedule
scheduler.bulk_schedule_messages(
    topic="notifications",
    messages=[{"patient_id": pid} for pid in patient_ids],
    scheduled_at=datetime(2024, 3, 15, 10, 0)  # Tomorrow 10am
)
```

### Daily Cron Job

```python
scheduler.create_schedule(ScheduleSpec(
    name="daily-reports",
    cron_expression="0 8 * * *",  # 8am daily
    topic="generate-report",
    message={"type": "daily"}
))
```

### Batch Messages for Efficiency

```python
# Instead of 10M rows, create 10K batch messages
for batch in chunk(patient_ids, 1000):
    scheduler.schedule_message(
        topic="notifications-batch",
        message={"patient_ids": batch},
        scheduled_at=target_time
    )
```

## Cron Expression Examples

```
* * * * *        # Every minute
0 * * * *        # Every hour
0 0 * * *        # Daily at midnight
0 9 * * 1-5      # Weekdays at 9am
0 0 1 * *        # First day of month
0 0 * * 0        # Every Sunday
*/15 * * * *     # Every 15 minutes
0 9-17 * * *     # Every hour from 9am to 5pm
```

## Environment Variables

```bash
DATABASE_URL=postgresql://user:pass@host/db
PUBSUB_PROJECT_ID=your-gcp-project
WORKER_ID=worker-1  # Optional, defaults to hostname
```

## Troubleshooting

### Messages Not Publishing

1. Check worker logs: `kubectl logs -f deployment/pubsub-scheduler-worker`
2. Verify database connection
3. Check scheduled_at is in the past: `SELECT COUNT(*) FROM scheduled_messages WHERE scheduled_at > NOW()`

### Duplicate Messages

- Should never happen with SKIP LOCKED
- Check for clock skew between workers and database
- Verify lease_duration is sufficient

### High Latency

- Increase worker replicas
- Adjust poll_interval (lower = faster)
- Increase batch_size (more messages per claim)

## Performance Tips

- **Bulk insert**: Use `bulk_schedule_messages()` for > 100 messages
- **Batch size**: Increase to 100-500 for higher throughput
- **Worker count**: Scale horizontally (20-100 workers)
- **Database**: Use Cloud SQL with connection pooling
- **Indexes**: Ensure migrations ran (indexes are critical)

## Architecture

```
App → Schedule Messages → Postgres (SKIP LOCKED) → Workers → Pub/Sub → Consumers
```

- Each message is a row in `scheduled_messages`
- Workers atomically claim messages (no conflicts)
- Published messages marked as `published`
- Failed messages auto-retry after lease expires

## Key Concepts

- **SKIP LOCKED**: PostgreSQL feature that prevents workers from blocking each other
- **Lease**: Time-based lock on messages (default 5 minutes)
- **Atomic claiming**: Each message claimed by exactly one worker
- **Crash safety**: Leases expire, messages become available again
- **Horizontal scaling**: Add more workers = linear throughput increase

## Links

- GitHub: https://github.com/fraser-isbester/pubsubscheduler
- Documentation: See README.md
- Comparison to Cron: See docs/comparison-to-cron.md
- 10M Messages Guide: See docs/10m-messages-example.md
