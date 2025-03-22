package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"encoding/json"
	"fmt"
	"time"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"strings"
)

// TODO delete
func validateQueueJob(job queue.Job) error {
	var missingFields []string
	if job.JobType == "" {
		missingFields = append(missingFields, "JobType")
	}
	if len(job.Payload) == 0 {
		missingFields = append(missingFields, "Payload")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("%w: %s", db.ErrMissingFields, strings.Join(missingFields, ", "))
	}
	return nil
}

func (d *Db) GetJobs(limit int) ([]*queue.Job, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var jobs []*queue.Job
	err := sqlitex.Exec(conn,
		`SELECT id, job_type, payload, status, attempts, max_attempts, created_at, updated_at, 
				scheduled_for, locked_by, locked_at, completed_at, last_error
		FROM job_queue
			WHERE status IN ('pending', 'failed')
		ORDER BY id ASC
		LIMIT ?`,
		func(stmt *sqlite.Stmt) error {
			createdAt, err := db.TimeParse(stmt.GetText("created_at"))
			if err != nil {
				return fmt.Errorf("error parsing created_at time: %w", err)
			}

			updatedAt, err := db.TimeParse(stmt.GetText("updated_at"))
			if err != nil {
				return fmt.Errorf("error parsing updated_at time: %w", err)
			}

			// following time.Time fields can be "" in the db. Only parse if not empty
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
		return nil, fmt.Errorf("failed to get jobs: %w", err)
	}
	return jobs, nil
}

func (d *Db) InsertJob(job queue.Job) error {
	if err := validateQueueJob(job); err != nil {
		return err
	}

	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	err := sqlitex.Exec(conn, `INSERT INTO job_queue 
		(job_type, payload, attempts, max_attempts) 
		VALUES (?, ?, ?, ?)`,
		nil,                 // No results needed for INSERT
		job.JobType,         // 1. job_type
		string(job.Payload), // 2. payload
		job.Attempts,        // 4. attempts
		job.MaxAttempts,     // 5. max_attempts
	)

	if err != nil {
		if sqliteErr, ok := err.(sqlite.Error); ok {
			if sqliteErr.Code == sqlite.SQLITE_CONSTRAINT_UNIQUE {
				return db.ErrConstraintUnique
			}
		}
		return fmt.Errorf("queue insert failed: %w", err)
	}
	return nil
}
