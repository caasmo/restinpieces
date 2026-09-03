package databasesql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/caasmo/restinpieces/db"
)

const (
	StmtClaim         = "claim"
	StmtInsertJob     = "insertJob"
	StmtMarkCompleted = "markCompleted"
	StmtMarkFailed    = "markFailed"
)

// QueueStmts maps the name of each queue statement to its SQL.
// The queue Db methods run these statements; restinpieces registers every
// entry once at startup through WithModerncPool, so each statement is
// prepared a single time before the first request.
var QueueStmts = map[string]string{
	StmtClaim: `UPDATE job_queue
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
			scheduled_for, locked_by, locked_at, completed_at, last_error, recurrent, interval`,

	StmtInsertJob: `INSERT INTO job_queue
		(job_type, payload, payload_extra, attempts, max_attempts, recurrent, interval, scheduled_for)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,

	StmtMarkCompleted: `UPDATE job_queue
		SET status = 'completed',
			completed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			locked_at = '',
			last_error = ''
		WHERE id = ?`,

	StmtMarkFailed: `UPDATE job_queue
		SET status = 'failed',
			updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
			locked_at = '',
			last_error = ?
		WHERE id = ?`,
}

// newJobFromRow creates a Job struct from a scanned database row.
func newJobFromRow(row *sql.Rows) (*db.Job, error) {
	var (
		job             db.Job
		payloadStr      string
		payloadExtraStr string
		recurrent       int64
		createdAtStr    string
		updatedAtStr    string
		scheduledForStr string
		lockedAtStr     string
		completedAtStr  string
		intervalStr     string
	)
	err := row.Scan(
		&job.ID, &job.JobType, &payloadStr, &payloadExtraStr, &job.Status,
		&job.Attempts, &job.MaxAttempts, &createdAtStr, &updatedAtStr,
		&scheduledForStr, &job.LockedBy, &lockedAtStr, &completedAtStr,
		&job.LastError, &recurrent, &intervalStr,
	)
	if err != nil {
		return nil, err
	}
	job.Payload = json.RawMessage(payloadStr)
	job.PayloadExtra = json.RawMessage(payloadExtraStr)
	job.Recurrent = recurrent != 0

	createdAt, err := db.TimeParse(createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing created_at time: %w", err)
	}
	job.CreatedAt = createdAt

	updatedAt, err := db.TimeParse(updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing updated_at time: %w", err)
	}
	job.UpdatedAt = updatedAt

	// Handle nullable time fields (empty strings mean zero time).
	if scheduledForStr != "" {
		scheduledFor, err := db.TimeParse(scheduledForStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing scheduled_for time: %w", err)
		}
		job.ScheduledFor = scheduledFor
	}
	if lockedAtStr != "" {
		lockedAt, err := db.TimeParse(lockedAtStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing locked_at time: %w", err)
		}
		job.LockedAt = lockedAt
	}
	if completedAtStr != "" {
		completedAt, err := db.TimeParse(completedAtStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing completed_at time: %w", err)
		}
		job.CompletedAt = completedAt
	}

	if intervalStr != "" {
		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing interval duration '%s': %w", intervalStr, err)
		}
		job.Interval = interval
	}

	return &job, nil
}

// insertJob performs the actual database insertion for a job using the
// provided prepared statement.
func insertJob(stmt *sql.Stmt, job db.Job) error {
	// Format ScheduledFor time if it's not zero
	var scheduledForStr string
	if !job.ScheduledFor.IsZero() {
		scheduledForStr = db.TimeFormat(job.ScheduledFor)
	}

	_, err := stmt.ExecContext(context.Background(),
		job.JobType, string(job.Payload), string(job.PayloadExtra), job.Attempts,
		job.MaxAttempts, job.Recurrent, job.Interval.String(), scheduledForStr)
	if err != nil {
		return fmt.Errorf("queue insert failed: %w", err)
	}
	return nil
}

// InsertJob adds a new job to the queue.
func (d *Db) InsertJob(job db.Job) error {
	stmt, err := d.Stmt(StmtInsertJob)
	if err != nil {
		return err
	}

	return insertJob(stmt, job)
}

// Claim locks and returns up to limit jobs for processing.
func (d *Db) Claim(limit int) (jobs []*db.Job, err error) {
	stmt, err := d.Stmt(StmtClaim)
	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to claim jobs: %w", err)
	}
	defer func() {
		err = errors.Join(err, rows.Close())
	}()

	for rows.Next() {
		job, rowErr := newJobFromRow(rows)
		if rowErr != nil {
			return nil, rowErr // Propagate parsing errors
		}
		jobs = append(jobs, job)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate claim results: %w", err)
	}

	// Return empty slice if no jobs were claimed, consistent with the
	// previous driver.
	if jobs == nil {
		jobs = []*db.Job{}
	}
	return jobs, nil
}

// markCompleted performs the actual database update for marking a job
// completed using the provided prepared statement.
func markCompleted(stmt *sql.Stmt, jobID int64) error {
	_, err := stmt.ExecContext(context.Background(), jobID)
	if err != nil {
		return fmt.Errorf("failed to mark job as completed: %w", err)
	}
	return nil
}

// MarkCompleted marks a job as completed successfully.
func (d *Db) MarkCompleted(jobID int64) error {
	stmt, err := d.Stmt(StmtMarkCompleted)
	if err != nil {
		return err
	}

	return markCompleted(stmt, jobID)
}

// markFailed performs the actual database update for marking a job failed
// using the provided prepared statement.
func markFailed(stmt *sql.Stmt, jobID int64, errMsg string) error {
	_, err := stmt.ExecContext(context.Background(), errMsg, jobID)
	if err != nil {
		return fmt.Errorf("failed to mark job as failed: %w", err)
	}
	return nil
}

// MarkFailed marks a job as failed.
func (d *Db) MarkFailed(jobID int64, errMsg string) error {
	stmt, err := d.Stmt(StmtMarkFailed)
	if err != nil {
		return err
	}

	return markFailed(stmt, jobID, errMsg)
}

// MarkRecurrentCompleted marks a job specified by completedJobID as completed
// and inserts the provided newJob within a single transaction.
//
// The transaction begins with BEGIN IMMEDIATE: a RESERVED lock is acquired
// immediately, which allows other connections to continue reading from the
// database, but it prevents any other connection from acquiring a RESERVED or
// EXCLUSIVE lock. This means no other connection can write to the database
// once the BEGIN IMMEDIATE succeeds.
//
// We cannot use StmtMarkCompleted and StmtInsertJob, the statements
// registered on startup: they are db.Prepare statements, so they run on
// whichever pool connection is free and cannot run inside this transaction.
// database/sql does not allow caching statements for use inside a
// transaction, so the two statements are prepared on the pinned connection
// with conn.PrepareContext instead. Conn-bound prepares are not cached:
// each call prepares them anew, at the same cost as executing the raw SQL.
func (d *Db) MarkRecurrentCompleted(completedJobID int64, newJob db.Job) (err error) {
	// Pin one connection: all transaction statements must run on it.
	conn, err := d.db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get connection for mark recurrent completed: %w", err)
	}
	defer func() {
		err = errors.Join(err, conn.Close())
	}()

	// Prepare both statements on the pinned connection. They are closed
	// before the connection returns to the pool, so nothing leaks on reuse.
	markStmt, err := conn.PrepareContext(context.Background(), QueueStmts[StmtMarkCompleted])
	if err != nil {
		return fmt.Errorf("failed to prepare mark completed statement: %w", err)
	}
	defer func() {
		err = errors.Join(err, markStmt.Close())
	}()

	insertStmt, err := conn.PrepareContext(context.Background(), QueueStmts[StmtInsertJob])
	if err != nil {
		return fmt.Errorf("failed to prepare insert job statement: %w", err)
	}
	defer func() {
		err = errors.Join(err, insertStmt.Close())
	}()

	// Execute both operations within a transaction
	if _, err = conn.ExecContext(context.Background(), "BEGIN IMMEDIATE;"); err != nil {
		return fmt.Errorf("failed to begin transaction for mark recurrent completed: %w", err)
	}

	// Defer rollback in case we exit early
	defer func() {
		if err != nil {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK;")
		}
	}()

	// Mark the specified job as completed
	if err = markCompleted(markStmt, completedJobID); err != nil {
		return fmt.Errorf("failed to mark job %d completed in transaction: %w", completedJobID, err)
	}

	// Insert the new job provided by the caller
	if err = insertJob(insertStmt, newJob); err != nil {
		return fmt.Errorf("failed to re-insert job in transaction: %w", err)
	}

	// Commit the transaction
	if _, err = conn.ExecContext(context.Background(), "COMMIT;"); err != nil {
		// Although the operations likely succeeded, the commit failed.
		// This is a problematic state, but we report the commit error.
		return fmt.Errorf("failed to commit transaction for mark recurrent completed: %w", err)
	}

	return nil
}
