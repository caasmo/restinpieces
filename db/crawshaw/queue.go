package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"encoding/json"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"time"
)

func (d *Db) InsertJob(job queue.Job) error {

	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	err := sqlitex.Exec(conn, `INSERT INTO job_queue
		(job_type, payload, payload_extra, attempts, max_attempts)
		VALUES (?, ?, ?, ?, ?)`,
		nil,                      // No results needed for INSERT
		job.JobType,              // 1. job_type
		string(job.Payload),      // 2. payload
		string(job.PayloadExtra), // 3. payload_extra
		job.Attempts,             // 4. attempts
		job.MaxAttempts,          // 5. max_attempts
	)

	if err != nil {
		// Removed specific unique constraint check
		return fmt.Errorf("queue insert failed: %w", err)
	}
	return nil
}

// Claim locks and returns up to limit jobs for processing
// The jobs are marked as 'processing' and locked by the current worker
func (d *Db) MarkCompleted(jobID int64) error {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	err := sqlitex.Exec(conn,
		`UPDATE job_queue
		SET status = 'completed',
			completed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			locked_at = '',
			last_error = ''
		WHERE id = ?`,
		nil,
		jobID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark job as completed: %w", err)
	}
	return nil
}

func (d *Db) MarkFailed(jobID int64, errMsg string) error {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	err := sqlitex.Exec(conn,
		`UPDATE job_queue
		SET status = 'failed',
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			locked_at = '',
			last_error = ?
		WHERE id = ?`,
		nil,
		errMsg,
		jobID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark job as failed: %w", err)
	}
	return nil
}

func (d *Db) Claim(limit int) ([]*queue.Job, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var jobs []*queue.Job
	err := sqlitex.Exec(conn,
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
		func(stmt *sqlite.Stmt) error {
			createdAt, err := db.TimeParse(stmt.GetText("created_at"))
			if err != nil {
				return fmt.Errorf("error parsing created_at time: %w", err)
			}

			updatedAt, err := db.TimeParse(stmt.GetText("updated_at"))
			if err != nil {
				return fmt.Errorf("error parsing updated_at time: %w", err)
			}

			var scheduledFor time.Time
			if scheduledForStr := stmt.GetText("scheduled_for"); scheduledForStr != "" {
				scheduledFor, err = db.TimeParse(scheduledForStr)
				if err != nil {
					return fmt.Errorf("error parsing scheduled_for time: %w", err)
				}
			}

			var lockedAt time.Time
			if lockedAtStr := stmt.GetText("locked_at"); lockedAtStr != "" {
				lockedAt, err = db.TimeParse(lockedAtStr)
				if err != nil {
					return fmt.Errorf("error parsing locked_at time: %w", err)
				}
			}

			var completedAt time.Time
			if completedAtStr := stmt.GetText("completed_at"); completedAtStr != "" {
				completedAt, err = db.TimeParse(completedAtStr)
				if err != nil {
					return fmt.Errorf("error parsing completed_at time: %w", err)
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
				LockedBy:     stmt.GetText("locked_by"),
				LockedAt:     lockedAt,
				CompletedAt:  completedAt,
				LastError:    stmt.GetText("last_error"),
			}
			jobs = append(jobs, job)
			return nil
		}, limit)

	if err != nil {
		return nil, fmt.Errorf("failed to claim jobs: %w", err)
	}
	return jobs, nil
}
