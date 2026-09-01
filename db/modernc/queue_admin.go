package modernc

import (
	"fmt"

	"github.com/caasmo/restinpieces/db"
)

// ListJobs retrieves a list of all jobs from the database, ordered by
// creation time.
func (d *Db) ListJobs(limit int) (jobs []*db.Job, err error) {
	query := `SELECT id, job_type, payload, payload_extra, status, attempts, max_attempts, created_at, updated_at,
			scheduled_for, locked_by, locked_at, completed_at, last_error, recurrent, interval
		FROM job_queue ORDER BY created_at DESC, id DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	query += ";"

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query list jobs: %w", err)
	}
	defer func() {
		if ferr := rows.Close(); ferr != nil && err == nil {
			err = fmt.Errorf("failed to close list jobs results: %w", ferr)
		}
	}()

	for rows.Next() {
		var job *db.Job
		job, err = newJobFromRow(rows)
		if err != nil {
			return nil, err // error already contains context
		}
		jobs = append(jobs, job)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate list jobs results: %w", err)
	}

	return jobs, nil
}

// DeleteJob removes a job from the queue by its ID.
func (d *Db) DeleteJob(jobID int64) error {
	res, err := d.db.Exec("DELETE FROM job_queue WHERE id = ?;", jobID)
	if err != nil {
		return fmt.Errorf("failed to execute delete job statement: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read delete job result: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("no job found with ID %d", jobID)
	}

	return nil
}
