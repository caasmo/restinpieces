package zombiezen

import (
	"context"
	"fmt"

	"github.com/caasmo/restinpieces/db"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// ListJobs retrieves a list of all jobs from the database, ordered by creation time.
func (d *DB) ListJobs(limit int) ([]*db.Job, error) {
	conn, err := d.pool.Take(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get db connection for list jobs: %w", err)
	}
	defer d.pool.Put(conn)

	query := "SELECT * FROM job_queue ORDER BY created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	query += ";"

	stmt, err := conn.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement for list jobs: %w", err)
	}
	defer stmt.Finalize()

	var jobs []*db.Job
	for {
		hasRow, err := stmt.Step()
		if err != nil {
			return nil, fmt.Errorf("failed to step through list jobs results: %w", err)
		}
		if !hasRow {
			break
		}

		job, err := scanJob(stmt)
		if err != nil {
			return nil, err // error already contains context
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// DeleteJob removes a job from the queue by its ID.
func (d *DB) DeleteJob(jobID int64) error {
	conn, err := d.pool.Take(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get db connection for delete job: %w", err)
	}
	defer d.pool.Put(conn)

	err = sqlitex.Execute(conn, "DELETE FROM job_queue WHERE id = ?;", &sqlitex.ExecOptions{
		Args: []interface{}{jobID},
	})
	if err != nil {
		return fmt.Errorf("failed to execute delete job statement: %w", err)
	}

	if conn.Changes() == 0 {
		return fmt.Errorf("no job found with ID %d", jobID)
	}

	return nil
}

// scanJob is a helper function to scan a sqlite.Stmt into a db.Job struct.
func scanJob(stmt *sqlite.Stmt) (*db.Job, error) {
	job := &db.Job{
		ID:           stmt.GetInt64("id"),
		JobType:      stmt.GetText("job_type"),
		Payload:      []byte(stmt.GetText("payload")),
		PayloadExtra: []byte(stmt.GetText("payload_extra")),
		Status:       stmt.GetText("status"),
		LastError:    stmt.GetText("last_error"),
		Attempts:     int(stmt.GetInt64("attempts")),
		MaxAttempts:  int(stmt.GetInt64("max_attempts")),
		Recurrent:    stmt.GetInt64("recurrent") == 1,
	}

	var err error
	job.CreatedAt, err = db.TimeParse(stmt.GetText("created_at"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at for job %d: %w", job.ID, err)
	}
	job.ScheduledFor, err = db.TimeParse(stmt.GetText("scheduled_for"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse scheduled_for for job %d: %w", job.ID, err)
	}
	job.ClaimedAt, err = db.TimeParse(stmt.GetText("claimed_at"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse claimed_at for job %d: %w", job.ID, err)
	}
	job.CompletedAt, err = db.TimeParse(stmt.GetText("completed_at"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse completed_at for job %d: %w", job.ID, err)
	}

	return job, nil
}
