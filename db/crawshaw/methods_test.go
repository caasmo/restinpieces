package crawshaw

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// TestSchemaVersion ensures the embedded users.sql schema matches the known hash.
// To update after schema changes:
// 1. Run: sha256sum migrations/users.sql
// 2. Replace knownHash with the output hash
// 3. Verify test data still works with new schema
func TestSchemaVersion(t *testing.T) {
	currentHash := sha256.Sum256([]byte(migrations.UsersSchema))
	knownHash := "cd6c8992ee383a88b0e86754400afea6ef89cb7475339a898435395d208726fd" // Replace with output from sha256sum

	if hex.EncodeToString(currentHash[:]) != knownHash {
		t.Fatal("users.sql schema has changed - update tests and knownHash")
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

	// Table setup definitions
	type tableSetup struct {
		name    string
		schema  string
		inserts []string
	}

	tables := []tableSetup{
		{
			name:   "users",
			schema: migrations.UsersSchema,
			inserts: []string{
				`INSERT INTO users (id, email, name, password, created, updated, verified, tokenKey)
				 VALUES ('test123', 'existing@test.com', 'Test User', 'hash123', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z', FALSE, 'token_key_setup')`,
			},
		},
		{
			name:    "job_queue",
			schema:  migrations.JobQueueSchema,
			inserts: []string{}, // No initial inserts for job_queue
		},
	}

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

func TestCreateUser(t *testing.T) {
	testDB := setupDB(t)
	defer testDB.Close()

	tests := []struct {
		name        string
		user        db.User
		wantErr     bool
		errorType   error    // Expected error type
		checkFields []string // Fields to verify in returned user
	}{
		{
			name: "valid user creation",
			user: db.User{
				Email:    "new@test.com",
				Password: "hashed_password_123",
				Name:     "New User",
				TokenKey: "token_key_valid_user_creation",
			},
			wantErr:     false,
			checkFields: []string{"Email", "Name", "Password", "TokenKey"},
		},
		{
			name: "duplicate email",
			user: db.User{
				Email:    "existing@test.com", // Same email as test user created in setupDB()
				Password: "hashed_password_123",
				Name:     "Duplicate User",
				TokenKey: "token_key_duplicate_email",
			},
			wantErr:   true,
			errorType: db.ErrConstraintUnique,
		},
		{
			name: "missing email",
			user: db.User{
				Email:    "", // Empty email
				Password: "hashed_password_123",
				Name:     "Invalid User",
				TokenKey: "token_key_missing_email",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			user: db.User{
				Email:    "missingpass@test.com",
				Password: "", // Empty password
				Name:     "Invalid User",
				TokenKey: "token_key_missing_password",
			},
			wantErr: true,
		},
		{
			name: "missing token key",
			user: db.User{
				Email:    "missingtoken@test.com",
				Password: "hashed_password_123",
				Name:     "Invalid User",
				TokenKey: "", // Empty token key
			},
			wantErr: true,
		},
		{
			name: "duplicate token key",
			user: db.User{
				Email:    "duptoken@test.com",
				Password: "hashed_password_123",
				Name:     "Duplicate Token User",
				TokenKey: "token_key_setup", // Same token key as test user created in setupDB()
			},
			wantErr:   true,
			errorType: db.ErrConstraintUnique,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdUser, err := testDB.CreateUser(tt.user)
			t.Logf("error details: %v", createdUser)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
					return
				}

				if tt.errorType != nil && !errors.Is(err, tt.errorType) {
					t.Errorf("expected error type %v, got %v", tt.errorType, err)
				}
				return
			}

			if err != nil {
				t.Logf("error details: %v", err)
				t.Fatalf("unexpected error: %v", err)
			}

			if createdUser == nil {
				t.Fatal("expected user but got nil")
			}

			// Verify returned fields match input
			for _, field := range tt.checkFields {
				switch field {
				case "Email":
					if createdUser.Email != tt.user.Email {
						t.Errorf("Email mismatch: got %q, want %q", createdUser.Email, tt.user.Email)
					}
				case "Name":
					if createdUser.Name != tt.user.Name {
						t.Errorf("Name mismatch: got %q, want %q", createdUser.Name, tt.user.Name)
					}
				case "Password":
					if createdUser.Password != tt.user.Password {
						t.Errorf("Password mismatch: got %q, want %q", createdUser.Password, tt.user.Password)
					}
				case "TokenKey":
					if createdUser.TokenKey != tt.user.TokenKey {
						t.Errorf("TokenKey mismatch: got %q, want %q", createdUser.TokenKey, tt.user.TokenKey)
					}
				}
			}

			// Verify timestamps are set and valid
			if createdUser.Created.IsZero() || createdUser.Updated.IsZero() {
				t.Error("timestamps not set")
			}

			// Verify timestamps are recent
			if time.Since(createdUser.Created) > time.Minute {
				t.Error("created timestamp is too old")
			}
			if time.Since(createdUser.Updated) > time.Minute {
				t.Error("updated timestamp is too old")
			}

			// Verify user can be retrieved
			retrievedUser, err := testDB.GetUserByEmail(tt.user.Email)
			if err != nil {
				t.Fatalf("failed to retrieve created user: %v", err)
			}

			if retrievedUser.ID != createdUser.ID {
				t.Errorf("retrieved user ID mismatch: got %q, want %q", retrievedUser.ID, createdUser.ID)
			}
		})
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
		JobType:     "test_job",  // Same job type as initial insert
		Payload:     json.RawMessage(`{"key":"unique_value"}`),  // Same payload as initial insert
		Status:      queue.StatusPending,
		MaxAttempts: 3,
	}
	err := testDB.InsertQueueJob(dupJob)

	if err == nil {
		t.Error("expected error but got none")
		return
	}

	if !errors.Is(err, db.ErrConstraintUnique) {
		t.Errorf("expected error type %v, got %v", db.ErrConstraintUnique, err)
	}
}
