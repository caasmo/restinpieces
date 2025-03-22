package crawshaw

import (
	"encoding/json"
	"testing"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
)

func TestInsertQueueJobValid(t *testing.T) {
	testDB := setupDB(t)
	defer testDB.Close()

	tests := []struct {
		name    string
		job     queue.Job
		wantErr bool
	}{
		{
			name: "valid job",
			job: queue.Job{
				JobType:     "test_job",
				Payload:     json.RawMessage(`{"key":"unique_value"}`),
				Status:      queue.StatusPending,
				MaxAttempts: 3,
			},
			wantErr: false,
		},
		{
			name: "missing job type",
			job: queue.Job{
				JobType:     "",
				Payload:     json.RawMessage(`{"key":"value"}`),
				MaxAttempts: 3,
			},
			wantErr: true,
		},
		{
			name: "empty payload",
			job: queue.Job{
				JobType:     "test_job",
				Payload:     json.RawMessage(``),
				MaxAttempts: 3,
			},
			wantErr: true,
		},
		{
		// TODO
			name: "invalid max attempts",
			job: queue.Job{
				JobType:     "test_job",
				Payload:     json.RawMessage(`{"key":"value"}`),
				MaxAttempts: 0,
				Status:      queue.StatusPending,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := testDB.InsertJob(tt.job)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify job was inserted correctly
			conn := testDB.pool.Get(nil)
			defer testDB.pool.Put(conn)

			var retrievedJob queue.Job
			// TODO use Get
			err = sqlitex.Exec(conn,
				`SELECT job_type, payload, status, attempts, max_attempts 
				FROM job_queue WHERE payload = ? LIMIT 1`,
				func(stmt *sqlite.Stmt) error {
					retrievedJob = queue.Job{
						JobType:     stmt.GetText("job_type"),
						Payload:     json.RawMessage(stmt.GetText("payload")),
						Status:      stmt.GetText("status"),
						Attempts:    int(stmt.GetInt64("attempts")),
						MaxAttempts: int(stmt.GetInt64("max_attempts")),
					}
					return nil
				}, string(tt.job.Payload))

			if err != nil {
				t.Fatalf("failed to verify job insertion: %v", err)
			}

			if retrievedJob.JobType != tt.job.JobType {
				t.Errorf("JobType mismatch: got %q, want %q", retrievedJob.JobType, tt.job.JobType)
			}
			if retrievedJob.Status != tt.job.Status {
				t.Errorf("Status mismatch: got %q, want %q", retrievedJob.Status, tt.job.Status)
			}
			if retrievedJob.MaxAttempts != tt.job.MaxAttempts {
				t.Errorf("MaxAttempts mismatch: got %d, want %d", retrievedJob.MaxAttempts, tt.job.MaxAttempts)
			}
		})
	}
}

func TestInsertQueueJobDuplicate(t *testing.T) {
	testDB := setupDB(t)
	defer testDB.Close()

	// First insert with unique payload
	uniqueJob := queue.Job{
		JobType:     "test_job",
		Payload:     json.RawMessage(`{"key":"unique_value"}`),
		Status:      queue.StatusPending,
		MaxAttempts: 3,
	}

	if err := testDB.InsertJob(uniqueJob); err != nil {
		t.Fatalf("unexpected error on first insert: %v", err)
	}

	// Second insert with duplicate payload
	dupJob := queue.Job{
		JobType:     "test_job",                                // Same job type as initial insert
		Payload:     json.RawMessage(`{"key":"unique_value"}`), // Same payload as initial insert
		Status:      queue.StatusPending,
		MaxAttempts: 3,
	}
	err := testDB.InsertJob(dupJob)

	if err == nil {
		t.Error("expected error but got none")
		return
	}

	if err != db.ErrConstraintUnique {
		t.Errorf("expected error type %v, got %v", db.ErrConstraintUnique, err)
	}
}
