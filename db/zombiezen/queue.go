package zombiezen

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"strings"
	"time"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// validateQueueJob checks for required fields in a job before insertion.
// Copied from crawshaw implementation for consistency.
func validateQueueJob(job queue.Job) error {
	var missingFields []string
	if job.JobType == "" {
		missingFields = append(missingFields, "JobType")
	}
	// PayloadExtra is optional

	if len(missingFields) > 0 {
		return fmt.Errorf("%w: %s", db.ErrMissingFields, strings.Join(missingFields, ", "))
	}
	return nil
}

// newJobFromStmt creates a Job struct from a SQLite statement row.
func newJobFromStmt(stmt *sqlite.Stmt) (*queue.Job, error) {
	createdAt, err := db.TimeParse(stmt.GetText("created_at"))
	if err != nil {
		return nil, fmt.Errorf("error parsing created_at time: %w", err)
	}

	updatedAt, err := db.TimeParse(stmt.GetText("updated_at"))
	if err != nil {
		return nil, fmt.Errorf("error parsing updated_at time: %w", err)
	}

	// Handle nullable time fields
	var scheduledFor time.Time
	if scheduledForStr := stmt.GetText("scheduled_for"); scheduledForStr != "" {
		scheduledFor, err = db.TimeParse(scheduledForStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing scheduled_for time: %w", err)
		}
	}

	var lockedAt time.Time
	if lockedAtStr := stmt.GetText("locked_at"); lockedAtStr != "" {
		lockedAt, err = db.TimeParse(lockedAtStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing locked_at time: %w", err)
		}
	}

	var completedAt time.Time
	if completedAtStr := stmt.GetText("completed_at"); completedAtStr != "" {
		completedAt, err = db.TimeParse(completedAtStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing completed_at time: %w", err)
		}
	}

	job := &queue.Job{
		ID:           stmt.GetInt64("id"),
		JobType:      stmt.GetText("job_type"),
		Payload:      json.RawMessage(stmt.GetText("payload")),
		PayloadExtra: json.RawMessage(stmt.GetText("payload_extra")),
		Status:       stmt.GetText("status"),
		Attempts:     int(stmt.GetInt64("attempts")),
		MaxAttempts:  int(stmt.GetInt64("max_attempts")),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		ScheduledFor: scheduledFor,
		LockedBy:     stmt.GetText("locked_by"), // Note: locked_by is not set by Claim in crawshaw either
		LockedAt:     lockedAt,
		CompletedAt:  completedAt,
		LastError:    stmt.GetText("last_error"),
	}
	return job, nil
}

// InsertJob adds a new job to the queue.
func (d *Db) InsertJob(job queue.Job) error {
	if err := validateQueueJob(job); err != nil {
		return err
	}

	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("queue insert failed to get connection: %w", err)
	}
	defer d.pool.Put(conn)

	err = sqlitex.Execute(conn, `INSERT INTO job_queue
		(job_type, payload, payload_extra, attempts, max_attempts)
		VALUES (?, ?, ?, ?, ?)`,
		&sqlitex.ExecOptions{ // Use ExecOptions even for INSERT without results
			Args: []interface{}{
				job.JobType,              // 1. job_type
				string(job.Payload),      // 2. payload
				string(job.PayloadExtra), // 3. payload_extra
				job.Attempts,             // 4. attempts
				job.MaxAttempts,          // 5. max_attempts
			},
		})

	if err != nil {
		// Check for unique constraint violation, similar to crawshaw
		// Use value type assertion like crawshaw
		if sqliteErr, ok := err.(sqlite.Error); ok && sqliteErr.Code() == sqlite.ResultConstraintUnique {
			return db.ErrConstraintUnique
		}
		return fmt.Errorf("queue insert failed: %w", err)
	}
	return nil
}

// Claim locks and returns up to limit jobs for processing.
// The jobs are marked as 'processing'.
func (d *Db) Claim(limit int) ([]*queue.Job, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for claim: %w", err)
	}
	defer d.pool.Put(conn)

	var jobs []*queue.Job
	err = sqlitex.Execute(conn,
		`UPDATE job_queue
		SET status = 'processing',
			locked_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			attempts = attempts + 1
		WHERE id IN (
			SELECT id
			FROM job_queue
			WHERE status IN ('pending', 'failed')
			ORDER BY id ASC
			LIMIT ?
		)
		RETURNING id, job_type, payload, payload_extra, status, attempts, max_attempts, created_at, updated_at,
			scheduled_for, locked_by, locked_at, completed_at, last_error`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				job, err := newJobFromStmt(stmt)
				if err != nil {
					return err // Propagate parsing errors
				}
				jobs = append(jobs, job)
				return nil
			},
			Args: []interface{}{limit},
		})

	if err != nil {
		return nil, fmt.Errorf("failed to claim jobs: %w", err)
	}
	// Return empty slice if no jobs were claimed, consistent with crawshaw
	if jobs == nil {
		jobs = []*queue.Job{}
	}
	return jobs, nil
}

// MarkCompleted marks a job as completed successfully.
func (d *Db) MarkCompleted(jobID int64) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get connection for mark completed: %w", err)
	}
	defer d.pool.Put(conn)

	err = sqlitex.Execute(conn,
		`UPDATE job_queue
		SET status = 'completed',
			completed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			locked_at = '',
			last_error = ''
		WHERE id = ?`,
		&sqlitex.ExecOptions{
			Args: []interface{}{jobID},
		})

	if err != nil {
		return fmt.Errorf("failed to mark job as completed: %w", err)
	}
	return nil
}

// MarkFailed marks a job as failed.
func (d *Db) MarkFailed(jobID int64, errMsg string) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get connection for mark failed: %w", err)
	}
	defer d.pool.Put(conn)

	err = sqlitex.Execute(conn,
		`UPDATE job_queue
		SET status = 'failed',
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			locked_at = '',
			last_error = ?
		WHERE id = ?`,
		&sqlitex.ExecOptions{
			Args: []interface{}{errMsg, jobID},
		})

	if err != nil {
		return fmt.Errorf("failed to mark job as failed: %w", err)
	}
	return nil
}
