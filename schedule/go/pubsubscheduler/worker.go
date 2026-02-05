package pubsubscheduler

import (
	"cloud.google.com/go/pubsub"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// scheduledMessage represents a message to be published
type scheduledMessage struct {
	ID      string
	Topic   string
	Message map[string]interface{}
}

// RunWorker runs the worker loop that claims and publishes messages to Pub/Sub
// This is a blocking call that runs until the context is cancelled
func (s *Scheduler) RunWorker(ctx context.Context, opts WorkerOptions) error {
	log.Printf("Starting worker %s with batch_size=%d, lease_duration=%s, poll_interval=%s",
		s.workerID, opts.BatchSize, opts.LeaseDuration, opts.PollInterval)

	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %s shutting down", s.workerID)
			return ctx.Err()
		case <-ticker.C:
			if err := s.processBatch(ctx, opts); err != nil {
				log.Printf("Error processing batch: %v", err)
			}
		}
	}
}

// processBatch claims and publishes a batch of messages
func (s *Scheduler) processBatch(ctx context.Context, opts WorkerOptions) error {
	// Claim messages atomically using SKIP LOCKED
	messages, err := s.claimMessages(ctx, opts.BatchSize, opts.LeaseDuration)
	if err != nil {
		return fmt.Errorf("failed to claim messages: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	log.Printf("Worker %s claimed %d messages", s.workerID, len(messages))

	// Publish each message to Pub/Sub
	for _, msg := range messages {
		if err := s.publishMessage(ctx, msg); err != nil {
			log.Printf("Failed to publish message %s: %v", msg.ID, err)
			s.markMessageFailed(ctx, msg.ID, err.Error())
		} else {
			log.Printf("Successfully published message %s to topic %s", msg.ID, msg.Topic)
			s.markMessagePublished(ctx, msg.ID)
		}
	}

	return nil
}

// claimMessages atomically claims messages ready to be published
func (s *Scheduler) claimMessages(ctx context.Context, limit int, leaseDuration time.Duration) ([]scheduledMessage, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Atomic claim using SKIP LOCKED
	query := `
		UPDATE scheduled_messages
		SET
			status = 'processing',
			lease_expires_at = NOW() + $1::interval,
			worker_id = $2,
			started_at = NOW(),
			attempts = attempts + 1
		WHERE id IN (
			SELECT id FROM scheduled_messages
			WHERE scheduled_at <= NOW()
			  AND status = 'pending'
			  AND (lease_expires_at IS NULL OR lease_expires_at < NOW())
			ORDER BY scheduled_at ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, topic, message
	`

	rows, err := tx.QueryContext(ctx, query,
		fmt.Sprintf("%d seconds", int(leaseDuration.Seconds())),
		s.workerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to claim messages: %w", err)
	}
	defer rows.Close()

	var messages []scheduledMessage
	for rows.Next() {
		var msg scheduledMessage
		var messageJSON []byte

		if err := rows.Scan(&msg.ID, &msg.Topic, &messageJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(messageJSON, &msg.Message); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message: %w", err)
		}

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return messages, nil
}

// publishMessage publishes a message to Google Cloud Pub/Sub
func (s *Scheduler) publishMessage(ctx context.Context, msg scheduledMessage) error {
	topic := s.pubsubClient.Topic(msg.Topic)
	defer topic.Stop()

	// Convert message to JSON bytes
	data, err := json.Marshal(msg.Message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish to Pub/Sub
	result := topic.Publish(ctx, &pubsub.Message{
		Data: data,
	})

	// Wait for publish to complete
	_, err = result.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to publish to pubsub: %w", err)
	}

	return nil
}

// markMessagePublished marks a message as successfully published
func (s *Scheduler) markMessagePublished(ctx context.Context, messageID string) error {
	query := `
		UPDATE scheduled_messages
		SET status = 'published', published_at = NOW()
		WHERE id = $1
	`

	_, err := s.db.ExecContext(ctx, query, messageID)
	if err != nil {
		return fmt.Errorf("failed to mark message as published: %w", err)
	}

	return nil
}

// markMessageFailed marks a message as failed
func (s *Scheduler) markMessageFailed(ctx context.Context, messageID string, errorMsg string) error {
	query := `
		UPDATE scheduled_messages
		SET status = 'failed', error_message = $1
		WHERE id = $2
	`

	_, err := s.db.ExecContext(ctx, query, errorMsg, messageID)
	if err != nil {
		return fmt.Errorf("failed to mark message as failed: %w", err)
	}

	return nil
}

// RunScheduleProcessor runs the schedule processor that spawns messages from recurring schedules
// This is a blocking call that runs until the context is cancelled
func (s *Scheduler) RunScheduleProcessor(ctx context.Context, interval time.Duration) error {
	log.Printf("Starting schedule processor with interval=%s", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Schedule processor shutting down")
			return ctx.Err()
		case <-ticker.C:
			if err := s.processSchedules(ctx); err != nil {
				log.Printf("Error processing schedules: %v", err)
			}
		}
	}
}

// processSchedules finds schedules ready to run and spawns messages
func (s *Scheduler) processSchedules(ctx context.Context) error {
	// Find schedules ready to run
	query := `
		SELECT id, name, cron_expression, topic, message, next_run_at
		FROM schedules
		WHERE enabled = true AND next_run_at <= NOW()
		ORDER BY next_run_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query schedules: %w", err)
	}
	defer rows.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	for rows.Next() {
		var scheduleID, name, cronExpression, topic string
		var messageJSON []byte
		var nextRunAt time.Time

		err := rows.Scan(&scheduleID, &name, &cronExpression, &topic, &messageJSON, &nextRunAt)
		if err != nil {
			log.Printf("Failed to scan schedule: %v", err)
			continue
		}

		// Parse message
		var message map[string]interface{}
		if err := json.Unmarshal(messageJSON, &message); err != nil {
			log.Printf("Failed to unmarshal message for schedule %s: %v", scheduleID, err)
			continue
		}

		// Spawn message
		if err := s.spawnScheduledMessage(ctx, scheduleID, topic, message, nextRunAt); err != nil {
			log.Printf("Failed to spawn message for schedule %s: %v", scheduleID, err)
			continue
		}

		// Calculate next run time
		schedule, err := parser.Parse(cronExpression)
		if err != nil {
			log.Printf("Failed to parse cron expression for schedule %s: %v", scheduleID, err)
			continue
		}

		nextRun := schedule.Next(time.Now())

		// Update schedule
		updateQuery := `
			UPDATE schedules
			SET last_run_at = $1, next_run_at = $2, updated_at = NOW()
			WHERE id = $3
		`
		_, err = s.db.ExecContext(ctx, updateQuery, nextRunAt, nextRun, scheduleID)
		if err != nil {
			log.Printf("Failed to update schedule %s: %v", scheduleID, err)
			continue
		}

		log.Printf("Spawned message for schedule %s (%s), next run at %s", scheduleID, name, nextRun)
	}

	return rows.Err()
}

// spawnScheduledMessage creates a scheduled message from a recurring schedule
func (s *Scheduler) spawnScheduledMessage(ctx context.Context, scheduleID, topic string, message map[string]interface{}, scheduledAt time.Time) error {
	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	query := `
		INSERT INTO scheduled_messages (schedule_id, topic, message, scheduled_at, status)
		VALUES ($1, $2, $3, $4, 'pending')
	`

	_, err = s.db.ExecContext(ctx, query, scheduleID, topic, messageJSON, scheduledAt)
	if err != nil {
		return fmt.Errorf("failed to insert scheduled message: %w", err)
	}

	return nil
}
