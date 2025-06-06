package zombiezen

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"time"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newJobFromStmt creates a Job struct from a SQLite statement row.
func newJobFromStmt(stmt *sqlite.Stmt) (*db.Job, error) {
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

	var interval time.Duration
	if intervalStr := stmt.GetText("interval"); intervalStr != "" {
		interval, err = time.ParseDuration(intervalStr)
		if err != nil {
			// Consider how to handle invalid duration strings from DB. Log? Error?
			// For now, return error to signal data inconsistency.
			return nil, fmt.Errorf("error parsing interval duration '%s': %w", intervalStr, err)
		}
	}

	job := &db.Job{
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
		Recurrent:    stmt.GetInt64("recurrent") != 0, // Convert SQLite INTEGER (0/1) to bool
		Interval:     interval,
	}
	return job, nil
}

// insertJob performs the actual database insertion for a job using a provided connection.
func (d *Db) insertJob(conn *sqlite.Conn, job db.Job) error {
	// Format ScheduledFor time if it's not zero
	var scheduledForStr string
	if !job.ScheduledFor.IsZero() {
		scheduledForStr = db.TimeFormat(job.ScheduledFor)
	}

	err := sqlitex.Execute(conn, `INSERT INTO job_queue
		(job_type, payload, payload_extra, attempts, max_attempts, recurrent, interval, scheduled_for)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []interface{}{
				job.JobType,              // 1. job_type
				string(job.Payload),      // 2. payload
				string(job.PayloadExtra), // 3. payload_extra
				job.Attempts,             // 4. attempts
				job.MaxAttempts,          // 5. max_attempts
				job.Recurrent,            // 6. recurrent
				job.Interval.String(),    // 7. interval (convert duration to string)
				scheduledForStr,          // 8. scheduled_for
			},
		})

	if err != nil {
		return fmt.Errorf("queue insert failed: %w", err)
	}
	return nil
}

// InsertJob adds a new job to the queue, acquiring its own connection.
func (d *Db) InsertJob(job db.Job) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("queue insert failed to get connection: %w", err)
	}
	defer d.pool.Put(conn)

	return d.insertJob(conn, job)
}

// Claim locks and returns up to limit jobs for processing.
func (d *Db) Claim(limit int) ([]*db.Job, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to get connection for claim: %w", err)
	}
	defer d.pool.Put(conn)

	var jobs []*db.Job
	// Note on scheduled_for comparison:
	// We compare scheduled_for with the current time using SQLite's string comparison.
	// This works because RFC3339 timestamps ('YYYY-MM-DDTHH:MM:SSZ') are lexicographically sortable.
	// An empty scheduled_for string (meaning schedule immediately) will correctly compare as less than
	// the current time string, ensuring these jobs are picked up.
	sql := `UPDATE job_queue
		SET status = 'processing',
			locked_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			attempts = attempts + 1
		WHERE id IN (
			SELECT id
			FROM job_queue
			WHERE status IN ('pending', 'failed')
			  -- Only claim jobs scheduled for now or in the past.
			  AND scheduled_for <= strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
			ORDER BY id ASC -- Maintain FIFO for due jobs
			LIMIT ?
		)
		RETURNING id, job_type, payload, payload_extra, status, attempts, max_attempts, created_at, updated_at,
			scheduled_for, locked_by, locked_at, completed_at, last_error, recurrent, interval` // Added recurrent, interval

	err = sqlitex.Execute(conn, sql, // Use the updated SQL string
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
		jobs = []*db.Job{}
	}
	return jobs, nil
}

// markCompleted performs the actual database update for marking a job completed using a provided connection.
func (d *Db) markCompleted(conn *sqlite.Conn, jobID int64) error {
	err := sqlitex.Execute(conn,
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

// MarkCompleted marks a job as completed successfully, acquiring its own connection.
func (d *Db) MarkCompleted(jobID int64) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get connection for mark completed: %w", err)
	}
	defer d.pool.Put(conn)

	return d.markCompleted(conn, jobID)
}

// markFailed performs the actual database update for marking a job failed using a provided connection.
func (d *Db) markFailed(conn *sqlite.Conn, jobID int64, errMsg string) error {
	err := sqlitex.Execute(conn,
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

// MarkFailed marks a job as failed, acquiring its own connection.
func (d *Db) MarkFailed(jobID int64, errMsg string) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get connection for mark failed: %w", err)
	}
	defer d.pool.Put(conn)

	return d.markFailed(conn, jobID, errMsg)
}

// MarkRecurrentCompleted marks a job specified by completedJobID as completed
// and inserts the provided newJob within a single transaction.
func (d *Db) MarkRecurrentCompleted(completedJobID int64, newJob db.Job) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get connection for mark recurrent completed: %w", err)
	}
	defer d.pool.Put(conn)

	// Execute both operations within a transaction
	// IMMEDIATE Starts a transaction and immediately attempts to acquire a
	// RESERVED lock on the database. A RESERVED lock allows other connections
	// to continue reading from the database, but it prevents any other
	// connection from acquiring a RESERVED or EXCLUSIVE lock. This means no
	// other connection can write to the database once the BEGIN IMMEDIATE
	// succeeds.
	err = sqlitex.Execute(conn, "BEGIN IMMEDIATE;", nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for mark recurrent completed: %w", err)
	}

	// Mark the specified job as completed
	err = d.markCompleted(conn, completedJobID)
	if err != nil {
		// Attempt to rollback
		_ = sqlitex.Execute(conn, "ROLLBACK;", nil)
		return fmt.Errorf("failed to mark job %d completed in transaction: %w", completedJobID, err)
	}

	// Insert the new job provided by the caller
	err = d.insertJob(conn, newJob)
	if err != nil {
		// Attempt to rollback
		_ = sqlitex.Execute(conn, "ROLLBACK;", nil)
		return fmt.Errorf("failed to re-insert job in transaction: %w", err)
	}

	// Commit the transaction
	err = sqlitex.Execute(conn, "COMMIT;", nil)
	if err != nil {
		// Although the operations likely succeeded, the commit failed.
		// This is a problematic state, but we report the commit error.
		return fmt.Errorf("failed to commit transaction for mark recurrent completed: %w", err)
	}

	return nil
}
