# Handling 10M Messages: A Step-by-Step Guide

This guide walks through a real-world scenario: **sending 10 million patient notifications at 10am tomorrow**.

## The Challenge

Your application needs to:
1. Send a notification to 10 million patients
2. At exactly 10am tomorrow
3. Without duplicates
4. With full crash safety
5. Track progress in real-time

Traditional approaches fail here:
- **Cron job**: Single pod, takes hours, crashes lose progress
- **Direct Pub/Sub publish**: No scheduling, no durability
- **In-memory queue**: Crashes lose everything

**PubSub Scheduler** solves this by making each message durable and independently claimable.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│  Step 1: Your App queries patient service                   │
│          Returns 10M patient IDs                             │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 2: Bulk schedule 10M messages to Postgres             │
│          INSERT INTO scheduled_messages (10M rows)           │
│          scheduled_at = '2024-03-15 10:00:00'                │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 3 (at 10am): 20 worker pods claim messages            │
│          Each worker atomically claims 100 messages          │
│          using SKIP LOCKED (no conflicts)                    │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 4: Workers publish to Pub/Sub                         │
│          topic: patient-notifications                        │
│          Each message: {"patient_id": "...", "type": "..."}  │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 5: Your existing Pub/Sub consumer sends notifications │
│          App notifications, emails, SMS, etc.                │
└─────────────────────────────────────────────────────────────┘
```

---

## Step-by-Step Implementation

### Step 1: Set Up Database

First, deploy PostgreSQL (Cloud SQL recommended for production):

```bash
# Create Cloud SQL instance
gcloud sql instances create pubsub-scheduler-db \
  --database-version=POSTGRES_15 \
  --tier=db-n1-standard-4 \
  --region=us-central1

# Create database
gcloud sql databases create scheduler --instance=pubsub-scheduler-db

# Run migration
psql $DATABASE_URL < migrations/001_create_pubsub_scheduler.sql
```

### Step 2: Bulk Schedule Messages

This is where you prepare the 10M messages. You run this **once**, ahead of time (e.g., today at 3pm).

#### Option A: Individual Messages (Full Durability)

```python
from datetime import datetime
from pubsubscheduler import Scheduler
from google.cloud import pubsub_v1

# Initialize
pubsub_client = pubsub_v1.PublisherClient()
scheduler = Scheduler(
    db_url="postgresql://user:pass@host/db",
    pubsub_client=pubsub_client
)

# Get eligible patients from your service
patient_ids = patients_service.get_eligible_patients()
print(f"Fetched {len(patient_ids)} patients")

# Bulk schedule all 10M messages
# This takes ~5-10 minutes to insert 10M rows
scheduler.bulk_schedule_messages(
    topic="patient-notifications",
    messages=[
        {
            "patient_id": pid,
            "notification_type": "daily_reminder",
            "timestamp": "2024-03-15T10:00:00Z"
        }
        for pid in patient_ids
    ],
    scheduled_at=datetime(2024, 3, 15, 10, 0, 0)  # Tomorrow at 10am
)

print("✅ Scheduled 10M messages for tomorrow at 10am")
```

**Database result**:
```sql
SELECT COUNT(*), status FROM scheduled_messages GROUP BY status;
-- count     | status
-- 10,000,000 | pending
```

**Pros**:
- Every message is durable and independently tracked
- Full crash safety (each message is atomic)
- Easy to query progress (`SELECT COUNT(*) WHERE status = 'published'`)

**Cons**:
- 10M rows in Postgres (but perfectly fine with proper indexing)

---

#### Option B: Batch Messages (Lighter)

If 10M rows feels too heavy, schedule **batch messages** instead:

```python
# Schedule 10K batch messages (each batch = 1000 patients)
# This creates only 10K rows instead of 10M

def chunk(lst, size):
    for i in range(0, len(lst), size):
        yield lst[i:i+size]

for batch in chunk(patient_ids, 1000):
    scheduler.schedule_message(
        topic="patient-notifications-batch",
        message={
            "patient_ids": batch,  # List of 1000 IDs
            "notification_type": "daily_reminder"
        },
        scheduled_at=datetime(2024, 3, 15, 10, 0, 0)
    )

print("✅ Scheduled 10K batch messages (covering 10M patients)")
```

**Your Pub/Sub consumer** then unpacks each batch:

```python
# Consumer code (already listening to patient-notifications-batch)
def handle_batch_message(message):
    data = json.loads(message.data)
    patient_ids = data["patient_ids"]  # 1000 IDs

    # Fan out to individual notifications
    for patient_id in patient_ids:
        send_notification_to_patient(patient_id)
```

**Pros**:
- Only 10K rows in Postgres (very light)
- Still durable and crash-safe

**Cons**:
- Less granular tracking (1000 patients per message)
- Consumer needs to handle batching logic

---

### Step 3: Deploy Workers to GKE

Workers are stateless pods that claim and publish messages. You deploy them **once** and they run continuously.

#### Create Kubernetes Secret

```bash
kubectl create secret generic postgres-creds \
  --from-literal=url="postgresql://user:pass@host/db"
```

#### Deploy Workers (Python Example)

Create `Dockerfile`:

```dockerfile
FROM python:3.11-slim

WORKDIR /app

COPY python/pubsubscheduler /app/pubsubscheduler
COPY python/examples/main.py /app/main.py
COPY python/setup.py /app/setup.py

RUN pip install -e .

CMD ["python", "main.py"]
```

Build and push:

```bash
docker build -t gcr.io/your-project/pubsub-scheduler-worker:latest .
docker push gcr.io/your-project/pubsub-scheduler-worker:latest
```

Deploy with `deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pubsub-scheduler-worker
spec:
  replicas: 20  # Start with 20 workers
  selector:
    matchLabels:
      app: pubsub-scheduler-worker
  template:
    metadata:
      labels:
        app: pubsub-scheduler-worker
    spec:
      containers:
      - name: worker
        image: gcr.io/your-project/pubsub-scheduler-worker:latest
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: postgres-creds
              key: url
        - name: PUBSUB_PROJECT_ID
          value: "your-gcp-project"
        - name: WORKER_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
```

Apply:

```bash
kubectl apply -f deployment.yaml
```

Verify workers are running:

```bash
kubectl get pods -l app=pubsub-scheduler-worker

# NAME                                      READY   STATUS    RESTARTS   AGE
# pubsub-scheduler-worker-7d8f4c5b6-8xkqz   1/1     Running   0          30s
# pubsub-scheduler-worker-7d8f4c5b6-9lmqz   1/1     Running   0          30s
# ... (20 pods total)
```

---

### Step 4: Monitor Progress

#### Real-Time Dashboard

Query Postgres to see progress:

```sql
-- Overall status
SELECT status, COUNT(*), ROUND(COUNT(*) * 100.0 / 10000000, 2) as percentage
FROM scheduled_messages
GROUP BY status;

-- status      | count     | percentage
-- pending     | 7,500,000 | 75.00
-- processing  | 50,000    | 0.50
-- published   | 2,400,000 | 24.00
-- failed      | 50,000    | 0.50

-- Worker activity
SELECT worker_id, COUNT(*), MAX(started_at) as last_active
FROM scheduled_messages
WHERE status = 'processing'
GROUP BY worker_id
ORDER BY COUNT(*) DESC;

-- worker_id                            | count | last_active
-- pubsub-scheduler-worker-7d8f4c5b6... | 100   | 2024-03-15 10:05:32
-- pubsub-scheduler-worker-7d8f4c5b6... | 100   | 2024-03-15 10:05:31
```

#### Check Worker Logs

```bash
kubectl logs -f deployment/pubsub-scheduler-worker

# 2024-03-15 10:00:01 - INFO - Starting worker pubsub-scheduler-worker-7d8f4c5b6-8xkqz
# 2024-03-15 10:00:01 - INFO - Worker claimed 100 messages
# 2024-03-15 10:00:02 - INFO - Successfully published message a1b2c3... to topic patient-notifications
# 2024-03-15 10:00:02 - INFO - Successfully published message d4e5f6... to topic patient-notifications
```

---

### Step 5: Handle Failures

#### Retry Failed Messages

If some messages fail (e.g., Pub/Sub rate limits), manually retry:

```sql
-- Reset failed messages to pending
UPDATE scheduled_messages
SET status = 'pending', lease_expires_at = NULL, error_message = NULL
WHERE status = 'failed';

-- Workers will automatically pick them up
```

Or implement automatic retries with exponential backoff:

```python
# In worker code
def _mark_message_failed(self, message_id: str, error_msg: str) -> None:
    conn = self._get_connection()
    with conn.cursor() as cur:
        # Check retry count
        cur.execute("SELECT attempts FROM scheduled_messages WHERE id = %s", (message_id,))
        attempts = cur.fetchone()[0]

        if attempts < 3:  # Max 3 retries
            # Reset to pending with exponential backoff
            retry_delay = 2 ** attempts  # 2s, 4s, 8s
            cur.execute(
                """
                UPDATE scheduled_messages
                SET status = 'pending',
                    lease_expires_at = NULL,
                    scheduled_at = NOW() + %s::interval
                WHERE id = %s
                """,
                (f"{retry_delay} seconds", message_id)
            )
        else:
            # Mark as permanently failed
            cur.execute(
                "UPDATE scheduled_messages SET status = 'failed', error_message = %s WHERE id = %s",
                (error_msg, message_id)
            )
    conn.commit()
```

#### Handle Worker Crashes

Workers are crash-safe by design:

```bash
# Simulate crash: kill a worker pod
kubectl delete pod pubsub-scheduler-worker-7d8f4c5b6-8xkqz

# Kubernetes immediately restarts the pod
# Meanwhile, other workers continue processing
# After 5 minutes, the dead worker's leases expire
# Other workers automatically pick up those messages
```

**No manual intervention needed.**

---

## Performance Metrics

### Baseline: 20 Workers

- **Batch size**: 100 messages per claim
- **Lease duration**: 5 minutes
- **Poll interval**: 1 second
- **Publishing rate**: ~10 messages/second per worker

**Calculation**:
```
Total messages: 10,000,000
Workers: 20
Rate per worker: 10 msg/sec
Total rate: 20 × 10 = 200 msg/sec
Time to complete: 10,000,000 ÷ 200 = 50,000 seconds = ~14 hours
```

### Optimized: 50 Workers

Scale up workers:

```bash
kubectl scale deployment pubsub-scheduler-worker --replicas=50
```

**Calculation**:
```
Workers: 50
Total rate: 50 × 10 = 500 msg/sec
Time to complete: 10,000,000 ÷ 500 = 20,000 seconds = ~5.5 hours
```

### Maximum Throughput: 100 Workers + Batch Size 500

```yaml
# Adjust worker options in code
opts = WorkerOptions(
    batch_size=500,  # Claim 500 messages at once
    lease_duration=timedelta(minutes=10),
    poll_interval=timedelta(milliseconds=100)
)
```

```bash
kubectl scale deployment pubsub-scheduler-worker --replicas=100
```

**Calculation**:
```
Workers: 100
Batch size: 500 (reduces DB round-trips)
Rate per worker: ~50 msg/sec (with batching)
Total rate: 100 × 50 = 5,000 msg/sec
Time to complete: 10,000,000 ÷ 5,000 = 2,000 seconds = ~33 minutes
```

---

## Production Checklist

- [ ] Database indices created (see migration file)
- [ ] Cloud SQL instance sized appropriately (db-n1-standard-4 or higher)
- [ ] Connection pooling configured (pgBouncer recommended)
- [ ] Workers deployed with HPA (auto-scale based on pending messages)
- [ ] Monitoring set up (Prometheus + Grafana for metrics)
- [ ] Alerting configured (alert if pending messages > threshold)
- [ ] Pub/Sub topics created and consumers deployed
- [ ] Dry run completed with 1000 messages
- [ ] Rollback plan documented (disable schedule, reset pending messages)

---

## Frequently Asked Questions

### Q: Can I use this for real-time scheduling (< 1 second latency)?

Not recommended. Workers poll every 1 second by default. For sub-second latency, use Pub/Sub directly with scheduled delivery or Cloud Tasks.

### Q: What if I need to cancel scheduled messages?

```sql
DELETE FROM scheduled_messages WHERE status = 'pending' AND schedule_id = '...';
```

Or disable the recurring schedule:

```python
scheduler.disable_schedule(schedule_id)
```

### Q: How do I handle time zones?

Store `scheduled_at` in UTC (Postgres `TIMESTAMPTZ` handles this). Convert in your application:

```python
import pytz

# User wants "10am Pacific Time"
pacific = pytz.timezone("America/Los_Angeles")
local_time = pacific.localize(datetime(2024, 3, 15, 10, 0, 0))
utc_time = local_time.astimezone(pytz.utc)

scheduler.schedule_message(..., scheduled_at=utc_time)
```

### Q: Can I prioritize certain messages?

Yes, add a `priority` column to `scheduled_messages` and modify the claim query:

```sql
ORDER BY priority DESC, scheduled_at ASC
```

### Q: What happens if Postgres goes down?

Workers will fail to claim messages and log errors. Once Postgres recovers, workers automatically resume. No messages are lost (they're durable in Postgres).

### Q: How much does this cost?

**Compute**: 20 workers × $0.05/hour = $1/hour = ~$720/month
**Database**: Cloud SQL db-n1-standard-4 = ~$400/month
**Pub/Sub**: 10M messages/day × $0.00002 = $200/day = ~$6000/month

**Total**: ~$7,120/month for 10M messages/day

(Much cheaper than alternatives like Cloud Tasks at scale)

---

## Conclusion

With PubSub Scheduler, you can:
- ✅ Schedule 10M messages in minutes (bulk insert)
- ✅ Publish them in hours (parallel workers)
- ✅ Track progress in real-time (SQL queries)
- ✅ Handle crashes gracefully (lease-based recovery)
- ✅ Scale horizontally (add more pods)

All without writing complex coordination logic or distributed locks.
