# PubSub Scheduler

A lightweight, durable message scheduling library for Google Cloud Pub/Sub. Built for horizontal scalability and crash-safety using PostgreSQL with `SKIP LOCKED` for atomic message claiming.

## Why This Library?

Traditional cron jobs have fundamental limitations when running in containerized environments like GKE:

- **No message-level atomicity**: If a cron job fails halfway through processing 10M messages, you restart from scratch
- **No horizontal scaling**: One cron pod processes everything serially
- **No progress tracking**: You don't know which messages were processed before the crash
- **No crash safety**: Worker dies? Lose all progress

**PubSub Scheduler solves these problems** by bridging durable scheduling with your existing Pub/Sub work queues:

```
Your App → Schedule 10M messages → Postgres Queue → Worker Pods → Pub/Sub → Your Consumers
```

Each message is independently claimed, published, and tracked. Workers scale horizontally with HPA. Crashes don't lose progress.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   Your Application                           │
│  - Queries data source (e.g., patient service)              │
│  - Calls scheduler.bulk_schedule_messages()                  │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│              Postgres (Cloud SQL)                            │
│  - schedules: Recurring patterns (cron)                      │
│  - scheduled_messages: Durable message queue                 │
│  - SKIP LOCKED ensures atomic claiming                       │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│      Library Workers (GKE Deployment + HPA)                  │
│  - Atomically claim messages (no coordination needed)        │
│  - Publish to Pub/Sub                                        │
│  - Mark as published or retry on failure                     │
│  - Scale with HPA based on pending messages                  │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│           Google Cloud Pub/Sub                               │
│  - Your existing work queue pattern                          │
│  - Messages flow to your consumers                           │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│         Your Existing Pub/Sub Consumers                      │
│  - Process notifications                                     │
│  - Send to app/email/SMS                                     │
└─────────────────────────────────────────────────────────────┘
```

## Key Features

- ✅ **Atomic message claiming** via PostgreSQL `SKIP LOCKED`
- ✅ **Horizontal scalability** - Multiple workers process messages in parallel
- ✅ **Crash-safe** - Lease-based recovery ensures no lost messages
- ✅ **Progress tracking** - Know exactly which messages were published
- ✅ **Bulk operations** - Efficiently schedule 10M+ messages
- ✅ **Recurring schedules** - Cron-like scheduling with automatic message spawning
- ✅ **Zero coordination** - No leader election, no distributed locks
- ✅ **GKE-native** - Designed for Kubernetes with HPA support

## Installation

### Go

```bash
go get github.com/fraser-isbester/pubsubscheduler
```

### Python

```bash
pip install pubsubscheduler
```

## Database Setup

Run the migration to create the required tables:

```bash
psql $DATABASE_URL < migrations/001_create_pubsub_scheduler.sql
```

This creates two tables:
- `schedules` - Recurring schedules (cron patterns)
- `scheduled_messages` - Individual messages to publish

## Quick Start

### Go Example

```go
package main

import (
    "context"
    "database/sql"
    "log"
    "time"

    "cloud.google.com/go/pubsub"
    _ "github.com/lib/pq"

    "github.com/fraser-isbester/pubsubscheduler/pubsubscheduler"
)

func main() {
    // Connect to database
    db, err := sql.Open("postgres", "postgres://user:pass@localhost/db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create Pub/Sub client
    ctx := context.Background()
    pubsubClient, err := pubsub.NewClient(ctx, "your-project-id")
    if err != nil {
        log.Fatal(err)
    }
    defer pubsubClient.Close()

    // Create scheduler
    scheduler := pubsubscheduler.NewScheduler(db, pubsubClient, "worker-1")

    // Schedule a one-time message
    messageID, err := scheduler.ScheduleMessage(
        ctx,
        "patient-notifications",
        map[string]interface{}{
            "patient_id": "12345",
            "type": "reminder",
        },
        time.Now().Add(24 * time.Hour), // Tomorrow
    )
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Scheduled message: %s", messageID)

    // Run worker (blocking)
    opts := pubsubscheduler.DefaultWorkerOptions()
    if err := scheduler.RunWorker(ctx, opts); err != nil {
        log.Fatal(err)
    }
}
```

### Python Example

```python
from datetime import datetime, timedelta
from google.cloud import pubsub_v1
from pubsubscheduler import Scheduler, WorkerOptions

# Create Pub/Sub client
pubsub_client = pubsub_v1.PublisherClient()

# Create scheduler
scheduler = Scheduler(
    db_url="postgresql://user:pass@localhost/db",
    pubsub_client=pubsub_client,
    worker_id="worker-1"
)

# Schedule a one-time message
message_id = scheduler.schedule_message(
    topic="patient-notifications",
    message={
        "patient_id": "12345",
        "type": "reminder"
    },
    scheduled_at=datetime.now() + timedelta(days=1)  # Tomorrow
)
print(f"Scheduled message: {message_id}")

# Run worker (blocking)
scheduler.run_worker(WorkerOptions(
    batch_size=10,
    lease_duration=timedelta(minutes=5),
    poll_interval=timedelta(seconds=1)
))
```

## Common Use Cases

### 1. Schedule 10M Patient Notifications

**Problem**: Send 10 million patients a notification at 10am tomorrow.

**Solution**: Bulk schedule messages, let workers publish in parallel.

```python
from datetime import datetime
from pubsubscheduler import Scheduler

# Get eligible patients
patient_ids = patients_service.get_eligible_patients()  # Returns 10M IDs

# Bulk schedule (efficient insert)
scheduler.bulk_schedule_messages(
    topic="patient-notifications",
    messages=[
        {"patient_id": pid, "notification_type": "reminder"}
        for pid in patient_ids
    ],
    scheduled_at=datetime.fromisoformat("2024-03-15T10:00:00Z")
)
```

**At 10am tomorrow**:
- Workers atomically claim messages using `SKIP LOCKED`
- Publish to Pub/Sub in parallel
- Your existing Pub/Sub consumers send notifications
- If a worker crashes, other workers pick up unpublished messages

### 2. Recurring Daily Schedule

**Problem**: Send daily reminders at 10am every day.

**Solution**: Create a recurring schedule that spawns messages automatically.

```python
from pubsubscheduler import ScheduleSpec

# Create once, runs daily automatically
schedule_id = scheduler.create_schedule(ScheduleSpec(
    name="daily-patient-reminders",
    cron_expression="0 10 * * *",  # 10am daily
    topic="trigger-patient-reminders",
    message={"task": "send_reminders"},
    enabled=True
))
```

Then have a Pub/Sub consumer that:
1. Listens to `trigger-patient-reminders`
2. Queries patient service for eligible patients
3. Bulk schedules messages for immediate delivery

### 3. Batch Processing for Efficiency

**Problem**: 10M individual messages creates 10M rows in Postgres.

**Solution**: Schedule batch messages instead.

```python
# Schedule 10K batch messages instead of 10M individual messages
for batch in chunk(patient_ids, 1000):
    scheduler.schedule_message(
        topic="patient-notifications-batch",
        message={
            "patient_ids": batch,
            "notification_type": "reminder"
        },
        scheduled_at=datetime.fromisoformat("2024-03-15T10:00:00Z")
    )
```

Your Pub/Sub consumer unpacks each batch and fans out to 1000 individual notifications.

## GKE Deployment

### 1. Deploy Postgres (Cloud SQL)

Create a Cloud SQL instance and run the migration:

```bash
psql $DATABASE_URL < migrations/001_create_pubsub_scheduler.sql
```

### 2. Create Kubernetes Secret

```bash
kubectl create secret generic postgres-creds \
  --from-literal=url="postgresql://user:pass@host/db"
```

### 3. Deploy Workers

#### Go Workers

```bash
# Build and push Docker image
docker build -t gcr.io/your-project/pubsub-scheduler-worker:latest -f go/examples/Dockerfile .
docker push gcr.io/your-project/pubsub-scheduler-worker:latest

# Deploy to GKE
kubectl apply -f go/examples/k8s/deployment.yaml
```

#### Python Workers

```bash
# Build and push Docker image
docker build -t gcr.io/your-project/pubsub-scheduler-worker-python:latest -f python/examples/Dockerfile .
docker push gcr.io/your-project/pubsub-scheduler-worker-python:latest

# Deploy to GKE
kubectl apply -f python/examples/k8s/deployment.yaml
```

### 4. Monitor Workers

```bash
# Check worker pods
kubectl get pods -l app=pubsub-scheduler-worker

# View logs
kubectl logs -f deployment/pubsub-scheduler-worker

# Check pending messages in database
psql $DATABASE_URL -c "SELECT status, COUNT(*) FROM scheduled_messages GROUP BY status;"
```

## API Reference

### Scheduler

#### `NewScheduler(db, pubsubClient, workerID)` (Go) / `Scheduler(db_url, pubsub_client, worker_id)` (Python)

Create a new scheduler instance.

#### `ScheduleMessage(topic, message, scheduledAt)` / `schedule_message(topic, message, scheduled_at)`

Schedule a single message.

**Parameters**:
- `topic`: Pub/Sub topic name
- `message`: Message payload (map/dict)
- `scheduledAt` / `scheduled_at`: When to publish

**Returns**: Message ID

#### `BulkScheduleMessages(topic, messages, scheduledAt)` / `bulk_schedule_messages(topic, messages, scheduled_at)`

Bulk schedule many messages efficiently.

**Parameters**:
- `topic`: Pub/Sub topic name
- `messages`: List of message payloads
- `scheduledAt` / `scheduled_at`: When to publish

#### `CreateSchedule(spec)` / `create_schedule(spec)`

Create a recurring schedule.

**Parameters**:
- `spec`: `ScheduleSpec` with name, cron_expression, topic, message, enabled

**Returns**: Schedule ID

#### `RunWorker(opts)` / `run_worker(options)`

Run worker loop (blocking). Claims and publishes messages.

**Parameters**:
- `opts` / `options`: `WorkerOptions` with batch_size, lease_duration, poll_interval

#### `RunScheduleProcessor(interval)` / `run_schedule_processor(interval)`

Run schedule processor (blocking). Spawns messages from recurring schedules.

**Parameters**:
- `interval`: How often to check for schedules (e.g., 10 seconds)

## How It Works

### Atomic Message Claiming

Workers use this PostgreSQL query to atomically claim messages:

```sql
UPDATE scheduled_messages
SET
    status = 'processing',
    lease_expires_at = NOW() + $lease_duration,
    worker_id = $worker_id,
    started_at = NOW(),
    attempts = attempts + 1
WHERE id IN (
    SELECT id FROM scheduled_messages
    WHERE scheduled_at <= NOW()
      AND status = 'pending'
      AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
    ORDER BY scheduled_at ASC
    LIMIT $limit
    FOR UPDATE SKIP LOCKED  -- Key: no blocking, no conflicts
)
RETURNING id, topic, message;
```

**`SKIP LOCKED`** is the magic:
- Multiple workers run this query in parallel
- Each worker gets a different set of messages (no duplicates)
- No coordination or locking conflicts
- Dead workers' messages become available after lease expires

### Crash Safety

If a worker crashes while processing:
1. Its lease expires (default: 5 minutes)
2. Other workers see `lease_expires_at < NOW()` in the WHERE clause
3. Messages automatically become available for re-claiming
4. No manual intervention needed

### Horizontal Scaling

Just add more worker pods:

```bash
kubectl scale deployment pubsub-scheduler-worker --replicas=10
```

Or use HPA to auto-scale based on pending messages:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
spec:
  scaleTargetRef:
    name: pubsub-scheduler-worker
  minReplicas: 3
  maxReplicas: 50
  metrics:
  - type: External
    external:
      metric:
        name: custom.googleapis.com|scheduler|pending_messages
      target:
        type: AverageValue
        averageValue: "1000"
```

## Comparison to Traditional Cron

| Feature | GKE CronJob | PubSub Scheduler |
|---------|-------------|------------------|
| Message-level atomicity | ❌ No (all-or-nothing) | ✅ Yes (SKIP LOCKED) |
| Horizontal scaling | ❌ No (single pod) | ✅ Yes (multiple workers) |
| Crash recovery | ❌ Restart from scratch | ✅ Automatic (lease expiry) |
| Progress tracking | ❌ No visibility | ✅ Every message tracked |
| Handles 10M messages | ❌ Timeouts/restarts | ✅ Parallel processing |
| Duplicate messages | ⚠️ On restart | ✅ Exactly-once claiming |
| Worker coordination | ❌ N/A (single pod) | ✅ Not needed (SKIP LOCKED) |

## Configuration

### Worker Options

| Option | Default | Description |
|--------|---------|-------------|
| `batch_size` | 10 | Messages to claim per poll |
| `lease_duration` | 5 minutes | How long to hold lease |
| `poll_interval` | 1 second | How often to poll for messages |

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `PUBSUB_PROJECT_ID` | Yes | Google Cloud project ID |
| `WORKER_ID` | No | Unique worker ID (defaults to hostname) |

## Monitoring

### Database Queries

Check message status distribution:

```sql
SELECT status, COUNT(*)
FROM scheduled_messages
GROUP BY status;
```

Find stale leases (possible stuck workers):

```sql
SELECT worker_id, COUNT(*), MAX(lease_expires_at)
FROM scheduled_messages
WHERE status = 'processing' AND lease_expires_at < NOW()
GROUP BY worker_id;
```

Monitor schedule health:

```sql
SELECT name, enabled, next_run_at, last_run_at
FROM schedules
ORDER BY next_run_at;
```

### Metrics

Export to Prometheus/Cloud Monitoring:
- `scheduled_messages_pending`: Number of pending messages
- `scheduled_messages_processing`: Number of processing messages
- `scheduled_messages_published`: Number of published messages
- `scheduled_messages_failed`: Number of failed messages

## Troubleshooting

### Messages not being published

1. Check worker logs: `kubectl logs -f deployment/pubsub-scheduler-worker`
2. Verify workers are running: `kubectl get pods -l app=pubsub-scheduler-worker`
3. Check database connectivity: Workers log "Connected to database" on startup
4. Verify scheduled_at is in the past: `SELECT COUNT(*) FROM scheduled_messages WHERE scheduled_at > NOW() AND status = 'pending';`

### Duplicate publications

Should not happen with `SKIP LOCKED`, but if it does:
1. Check for clock skew between workers and database
2. Verify lease_duration is sufficient (increase if workers are slow)
3. Check database logs for deadlocks (should be none)

### Schedule processor not spawning messages

1. Check processor logs: `kubectl logs -f deployment/pubsub-scheduler-processor`
2. Verify schedules are enabled: `SELECT * FROM schedules WHERE enabled = true;`
3. Check next_run_at is in the past: `SELECT * FROM schedules WHERE next_run_at <= NOW();`
4. Verify cron expressions are valid

## License

MIT

## Contributing

Pull requests welcome!