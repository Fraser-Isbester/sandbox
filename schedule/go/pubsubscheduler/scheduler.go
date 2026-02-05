package pubsubscheduler

import (
	"cloud.google.com/go/pubsub"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// Scheduler manages scheduling and publishing messages to Google Cloud Pub/Sub
type Scheduler struct {
	db           *sql.DB
	pubsubClient *pubsub.Client
	workerID     string
}

// ScheduleSpec defines a recurring schedule
type ScheduleSpec struct {
	ID             string
	Name           string
	CronExpression string
	Topic          string
	Message        map[string]interface{}
	Enabled        bool
	NextRunAt      *time.Time
	LastRunAt      *time.Time
}

// WorkerOptions configures the worker behavior
type WorkerOptions struct {
	BatchSize     int           // How many messages to claim at once (default: 10)
	LeaseDuration time.Duration // How long to hold lease (default: 5 minutes)
	PollInterval  time.Duration // How often to poll (default: 1 second)
}

// DefaultWorkerOptions returns sensible defaults
func DefaultWorkerOptions() WorkerOptions {
	return WorkerOptions{
		BatchSize:     10,
		LeaseDuration: 5 * time.Minute,
		PollInterval:  1 * time.Second,
	}
}

// NewScheduler creates a new scheduler instance
func NewScheduler(db *sql.DB, pubsubClient *pubsub.Client, workerID string) *Scheduler {
	if workerID == "" {
		workerID = uuid.New().String()
	}
	return &Scheduler{
		db:           db,
		pubsubClient: pubsubClient,
		workerID:     workerID,
	}
}

// ScheduleMessage schedules a single Pub/Sub message
func (s *Scheduler) ScheduleMessage(ctx context.Context, topic string, message map[string]interface{}, scheduledAt time.Time) (string, error) {
	messageJSON, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}

	id := uuid.New().String()
	query := `
		INSERT INTO scheduled_messages (id, topic, message, scheduled_at, status)
		VALUES ($1, $2, $3, $4, 'pending')
	`

	_, err = s.db.ExecContext(ctx, query, id, topic, messageJSON, scheduledAt)
	if err != nil {
		return "", fmt.Errorf("failed to insert scheduled message: %w", err)
	}

	return id, nil
}

// BulkScheduleMessages efficiently schedules many messages at once
func (s *Scheduler) BulkScheduleMessages(ctx context.Context, topic string, messages []map[string]interface{}, scheduledAt time.Time) error {
	if len(messages) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO scheduled_messages (id, topic, message, scheduled_at, status)
		VALUES ($1, $2, $3, $4, 'pending')
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, message := range messages {
		messageJSON, err := json.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}

		id := uuid.New().String()
		_, err = stmt.ExecContext(ctx, id, topic, messageJSON, scheduledAt)
		if err != nil {
			return fmt.Errorf("failed to insert message: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CreateSchedule creates a recurring schedule
func (s *Scheduler) CreateSchedule(ctx context.Context, spec ScheduleSpec) (string, error) {
	// Validate cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(spec.CronExpression)
	if err != nil {
		return "", fmt.Errorf("invalid cron expression: %w", err)
	}

	// Calculate next run time
	nextRun := schedule.Next(time.Now())

	messageJSON, err := json.Marshal(spec.Message)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}

	id := uuid.New().String()
	query := `
		INSERT INTO schedules (id, name, cron_expression, topic, message, enabled, next_run_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = s.db.ExecContext(ctx, query, id, spec.Name, spec.CronExpression, spec.Topic, messageJSON, spec.Enabled, nextRun)
	if err != nil {
		return "", fmt.Errorf("failed to insert schedule: %w", err)
	}

	return id, nil
}

// UpdateSchedule updates an existing schedule
func (s *Scheduler) UpdateSchedule(ctx context.Context, id string, spec ScheduleSpec) error {
	// Validate cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(spec.CronExpression)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// Calculate next run time
	nextRun := schedule.Next(time.Now())

	messageJSON, err := json.Marshal(spec.Message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	query := `
		UPDATE schedules
		SET name = $1, cron_expression = $2, topic = $3, message = $4, enabled = $5, next_run_at = $6, updated_at = NOW()
		WHERE id = $7
	`

	result, err := s.db.ExecContext(ctx, query, spec.Name, spec.CronExpression, spec.Topic, messageJSON, spec.Enabled, nextRun, id)
	if err != nil {
		return fmt.Errorf("failed to update schedule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("schedule not found: %s", id)
	}

	return nil
}

// DisableSchedule disables a recurring schedule
func (s *Scheduler) DisableSchedule(ctx context.Context, id string) error {
	query := `UPDATE schedules SET enabled = false, updated_at = NOW() WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to disable schedule: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("schedule not found: %s", id)
	}

	return nil
}

// GetSchedule retrieves a schedule by ID
func (s *Scheduler) GetSchedule(ctx context.Context, id string) (*ScheduleSpec, error) {
	query := `
		SELECT id, name, cron_expression, topic, message, enabled, next_run_at, last_run_at
		FROM schedules
		WHERE id = $1
	`

	var spec ScheduleSpec
	var messageJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&spec.ID,
		&spec.Name,
		&spec.CronExpression,
		&spec.Topic,
		&messageJSON,
		&spec.Enabled,
		&spec.NextRunAt,
		&spec.LastRunAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query schedule: %w", err)
	}

	if err := json.Unmarshal(messageJSON, &spec.Message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &spec, nil
}

// ListSchedules retrieves all schedules
func (s *Scheduler) ListSchedules(ctx context.Context) ([]*ScheduleSpec, error) {
	query := `
		SELECT id, name, cron_expression, topic, message, enabled, next_run_at, last_run_at
		FROM schedules
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query schedules: %w", err)
	}
	defer rows.Close()

	var schedules []*ScheduleSpec
	for rows.Next() {
		var spec ScheduleSpec
		var messageJSON []byte

		err := rows.Scan(
			&spec.ID,
			&spec.Name,
			&spec.CronExpression,
			&spec.Topic,
			&messageJSON,
			&spec.Enabled,
			&spec.NextRunAt,
			&spec.LastRunAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal(messageJSON, &spec.Message); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message: %w", err)
		}

		schedules = append(schedules, &spec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return schedules, nil
}
