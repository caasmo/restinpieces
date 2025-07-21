package zombiezen

import (
	"context"
	"fmt"

	"github.com/caasmo/restinpieces/db"
	"zombiezen.com/go/sqlite/sqlitex"
)

// ListJobs retrieves a list of all jobs from the database, ordered by creation time.
func (d *Db) ListJobs(limit int) (jobs []*db.Job, err error) {
	conn, err := d.pool.Take(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get db connection for list jobs: %w", err)
	}
	defer d.pool.Put(conn)

	query := "SELECT * FROM job_queue ORDER BY created_at DESC, id DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	query += ";"

	stmt, err := conn.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement for list jobs: %w", err)
	}
	defer func() {
		if ferr := stmt.Finalize(); ferr != nil && err == nil {
			err = fmt.Errorf("failed to finalize statement: %w", ferr)
		}
	}()

	for {
		var hasRow bool
		hasRow, err = stmt.Step()
		if err != nil {
			return nil, fmt.Errorf("failed to step through list jobs results: %w", err)
		}
		if !hasRow {
			break
		}

		var job *db.Job
		job, err = newJobFromStmt(stmt)
		if err != nil {
			return nil, err // error already contains context
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// DeleteJob removes a job from the queue by its ID.
func (d *Db) DeleteJob(jobID int64) error {
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
