package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"strings"
)

func validateQueueJob(job queue.QueueJob) error {
	var missingFields []string
	if job.JobType == "" {
		missingFields = append(missingFields, "JobType")
	}
	if len(job.Payload) == 0 {
		missingFields = append(missingFields, "Payload")
	}
	if job.MaxAttempts < 1 {
		missingFields = append(missingFields, "MaxAttempts must be ≥1")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("%w: %s", db.ErrMissingFields, strings.Join(missingFields, ", "))
	}
	return nil
}

func (d *Db) InsertQueueJob(job queue.QueueJob) error {
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
