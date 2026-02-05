-- PubSub Scheduler Schema
-- Purpose: Durable message scheduling and publishing to Google Cloud Pub/Sub
-- Key features:
--   - Atomic message claiming via SKIP LOCKED
--   - Crash-safe with lease-based recovery
--   - Supports both recurring schedules and one-time messages

-- Recurring schedules that spawn Pub/Sub messages automatically
CREATE TABLE schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    cron_expression VARCHAR(255) NOT NULL,  -- e.g., "0 8 * * *" for daily at 8am
    topic VARCHAR(255) NOT NULL,            -- Pub/Sub topic name
    message JSONB NOT NULL,                 -- Message payload to publish
    enabled BOOLEAN DEFAULT true,
    next_run_at TIMESTAMPTZ,                -- When to next spawn message
    last_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for efficient polling of schedules ready to run
CREATE INDEX idx_schedules_next_run ON schedules(next_run_at) WHERE enabled = true;

-- Individual Pub/Sub messages to be published (the queue)
CREATE TABLE scheduled_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID REFERENCES schedules(id) ON DELETE SET NULL,  -- NULL for one-time messages
    topic VARCHAR(255) NOT NULL,            -- Pub/Sub topic name
    message JSONB NOT NULL,                 -- Message payload
    scheduled_at TIMESTAMPTZ NOT NULL,      -- When to publish this message
    status VARCHAR(50) NOT NULL DEFAULT 'pending',  -- pending, processing, published, failed
    lease_expires_at TIMESTAMPTZ,           -- Worker lease expiration (for crash recovery)
    worker_id VARCHAR(255),                 -- Which worker is processing this
    attempts INT DEFAULT 0,                 -- Track retry attempts (for observability)
    error_message TEXT,                     -- Last error if failed
    created_at TIMESTAMPTZ DEFAULT NOW(),
    started_at TIMESTAMPTZ,                 -- When processing started
    published_at TIMESTAMPTZ,               -- When successfully published to Pub/Sub
    CONSTRAINT valid_status CHECK (status IN ('pending', 'processing', 'published', 'failed'))
);

-- Critical index for worker polling: enables efficient SKIP LOCKED queries
-- This index supports the atomic message claiming query used by workers
CREATE INDEX idx_scheduled_messages_poll ON scheduled_messages(scheduled_at, status, lease_expires_at)
    WHERE status IN ('pending', 'processing');

-- Index for querying messages by schedule (useful for monitoring)
CREATE INDEX idx_scheduled_messages_schedule ON scheduled_messages(schedule_id);

-- Index for querying messages by topic (useful for debugging/monitoring)
CREATE INDEX idx_scheduled_messages_topic ON scheduled_messages(topic);
