package crawshaw

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/migrations"
	"github.com/caasmo/restinpieces/queue"
)

// Schema Hash Verification Process:
// 1. Any changes to migrations/users.sql will break this test
// 2. Calculate new hash with: sha256sum migrations/users.sql
// 3. Update knownHash in TestSchemaVersion with the new value
// 4. Review test data in setupDB() for compatibility with schema changes

type tableSchema struct {
	name      string
	schema    string
	inserts   []string
	knownHash string
}

var tables = []tableSchema{
	{
		name:   "users",
		schema: migrations.UsersSchema,
		inserts: []string{},
		knownHash: "a8442a840a7adb04578fe2f1b3a14debd9f669a3e7cd48eda8ff365cf027398d",
	},
	{
		name:      "job_queue",
		schema:    migrations.JobQueueSchema,
		inserts:   []string{},
		knownHash: "31807e9841313811ab08a2a4c5cd8df2f81c3fbc8f3c63af1bfb4045591b577a",
	},
}

// TestSchemaVersion ensures embedded schemas match known hashes.
// To update after schema changes:
// 1. Run: sha256sum migrations/<schema>.sql
// 2. Replace knownHash with the output hash
// 3. Verify test data still works with new schema
func TestSchemaVersion(t *testing.T) {

	for _, tbl := range tables {
		currentHash := sha256.Sum256([]byte(tbl.schema))
		if hex.EncodeToString(currentHash[:]) != tbl.knownHash {
			t.Fatalf("%s schema has changed - update tests and knownHash", tbl.name)
		}
	}
}

func setupDB(t *testing.T) *Db {
	t.Helper()

	// Using a named in-memory database with the URI format
	// file:testdb?mode=memory&cache=shared allows multiple connections to
	// access the same in-memory database
	pool, err := sqlitex.Open("file:testdb?mode=memory&cache=shared", 0, 4)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	conn := pool.Get(context.TODO())
	defer pool.Put(conn)

	// Use shared tables configuration

	// Process each table
	for _, tbl := range tables {
		// Drop table
		if err := sqlitex.ExecScript(conn, fmt.Sprintf("DROP TABLE IF EXISTS %s", tbl.name)); err != nil {
			t.Fatalf("failed to drop %s table: %v", tbl.name, err)
		}

		// Create table
		if err := sqlitex.ExecScript(conn, tbl.schema); err != nil {
			t.Fatalf("failed to create %s table: %v", tbl.name, err)
		}
	}

	// Insert test data after all tables are created
	for _, tbl := range tables {
		for _, insertSQL := range tbl.inserts {
			if err := sqlitex.ExecScript(conn, insertSQL); err != nil {
				t.Fatalf("failed to insert into %s table: %v", tbl.name, err)
			}
		}
	}
	if err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}

	// Return DB instance with the existing pool that has our schema
	return &Db{
		pool: pool,
		rwCh: make(chan *sqlite.Conn, 1),
	}
}


func TestGetUserByEmail(t *testing.T) {
	testDB := setupDB(t)
	defer testDB.Close()

	tests := []struct {
		name     string
		email    string
		wantUser *db.User
		wantErr  bool
	}{
		{
			name:  "existing user",
			email: "existing@test.com",
			wantUser: &db.User{
				ID:       "test123",
				Email:    "existing@test.com",
				Name:     "Test User",
				Password: "hash123",
				Created:  time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
				Updated:  time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
				Verified: false,
			},
			wantErr: false,
		},
		{
			name:     "non-existent user",
			email:    "nonexistent@test.com",
			wantUser: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := testDB.GetUserByEmail(tt.email)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
				return
			} else if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantUser != nil {
				if user == nil {
					t.Error("expected user but got nil")
					return
				}
				if user.ID != tt.wantUser.ID ||
					user.Email != tt.wantUser.Email ||
					user.Name != tt.wantUser.Name ||
					user.Password != tt.wantUser.Password ||
					user.Created != tt.wantUser.Created ||
					user.Updated != tt.wantUser.Updated ||
					user.Verified != tt.wantUser.Verified {
					t.Errorf("GetUserByEmail() = %+v, want %+v", user, tt.wantUser)
				}
			} else if user != nil {
				t.Error("expected nil user but got result")
			}
		})
	}
}

func TestInsertQueueJobValid(t *testing.T) {
	testDB := setupDB(t)
	defer testDB.Close()

	tests := []struct {
		name    string
		job     queue.QueueJob
		wantErr bool
	}{
		{
			name: "valid job",
			job: queue.QueueJob{
				JobType:     "test_job",
				Payload:     json.RawMessage(`{"key":"unique_value"}`),
				Status:      queue.StatusPending,
				MaxAttempts: 3,
			},
			wantErr: false,
		},
		{
			name: "missing job type",
			job: queue.QueueJob{
				JobType:     "",
				Payload:     json.RawMessage(`{"key":"value"}`),
				MaxAttempts: 3,
			},
			wantErr: true,
		},
		{
			name: "empty payload",
			job: queue.QueueJob{
				JobType:     "test_job",
				Payload:     json.RawMessage(``),
				MaxAttempts: 3,
			},
			wantErr: true,
		},
		{
			name: "invalid max attempts",
			job: queue.QueueJob{
				JobType:     "test_job",
				Payload:     json.RawMessage(`{"key":"value"}`),
				MaxAttempts: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := testDB.InsertQueueJob(tt.job)

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
			// TODO remoe this when select method implemented
			conn := testDB.pool.Get(nil)
			defer testDB.pool.Put(conn)

			var retrievedJob queue.QueueJob
			err = sqlitex.Exec(conn,
				`SELECT job_type, payload, status, attempts, max_attempts 
				FROM job_queue WHERE payload = ? LIMIT 1`,
				func(stmt *sqlite.Stmt) error {
					retrievedJob = queue.QueueJob{
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

func TestCreateUserWithPassword(t *testing.T) {
	testDB := setupDB(t)
	defer testDB.Close()

	// Test valid user creation
	t.Run("successful creation", func(t *testing.T) {
		user := db.User{
			Email:           "test@example.com",
			Password:        "hashed_password", 
			Name:            "Test User",
			Verified:        false,
			Oauth2:          false,
			Avatar:          "avatar.jpg",
			EmailVisibility: false,
		}

		createdUser, err := testDB.CreateUserWithPassword(user)
		if err != nil {
			t.Fatalf("CreateUserWithPassword failed: %v", err)
		}

		// Verify returned fields
		if createdUser.Email != user.Email {
			t.Errorf("Email mismatch: got %q, want %q", createdUser.Email, user.Email)
		}
		if createdUser.Password != user.Password {
			t.Errorf("Password mismatch: got %q, want %q", createdUser.Password, user.Password)
		}
		if createdUser.Name != user.Name {
			t.Errorf("Name mismatch: got %q, want %q", createdUser.Name, user.Name)
		}
		if createdUser.Verified != user.Verified {
			t.Errorf("Verified mismatch: got %v, want %v", createdUser.Verified, user.Verified)
		}
		if createdUser.Oauth2 != user.Oauth2 {
			t.Errorf("Oauth2 mismatch: got %v, want %v", createdUser.Oauth2, user.Oauth2)
		}

		// Verify timestamps
		if createdUser.Created.IsZero() {
			t.Error("Created timestamp not set")
		}
		if createdUser.Updated.IsZero() {
			t.Error("Updated timestamp not set")
		}
	})

	// Test email conflict with different password
	t.Run("email conflict with different password", func(t *testing.T) {
		// First create user
		user1 := db.User{
			Email:    "conflict@test.com",
			Password: "hash1",
		}
		_, err := testDB.CreateUserWithPassword(user1)
		if err != nil {
			t.Fatalf("Failed to create initial user: %v", err)
		}

		// Try to create user with same email but different password
		user2 := db.User{
			Email:    "conflict@test.com",
			Password: "hash2",
		}
		createdUser, err := testDB.CreateUserWithPassword(user2)
		if err != nil {
			t.Fatalf("CreateUserWithPassword failed: %v", err)
		}

		// Should return existing user with original password
		if createdUser.Password != user1.Password {
			t.Errorf("Password was updated, expected %q got %q", user1.Password, createdUser.Password)
		}
	})
}

func TestInsertQueueJobDuplicate(t *testing.T) {
	testDB := setupDB(t)
	defer testDB.Close()

	// First insert with unique payload
	uniqueJob := queue.QueueJob{
		JobType:     "test_job",
		Payload:     json.RawMessage(`{"key":"unique_value"}`),
		Status:      queue.StatusPending,
		MaxAttempts: 3,
	}

	if err := testDB.InsertQueueJob(uniqueJob); err != nil {
		t.Fatalf("unexpected error on first insert: %v", err)
	}

	// Second insert with duplicate payload
	dupJob := queue.QueueJob{
		JobType:     "test_job",                                // Same job type as initial insert
		Payload:     json.RawMessage(`{"key":"unique_value"}`), // Same payload as initial insert
		Status:      queue.StatusPending,
		MaxAttempts: 3,
	}
	err := testDB.InsertQueueJob(dupJob)

	if err == nil {
		t.Error("expected error but got none")
		return
	}

	if err != db.ErrConstraintUnique {
		t.Errorf("expected error type %v, got %v", db.ErrConstraintUnique, err)
	}
}
