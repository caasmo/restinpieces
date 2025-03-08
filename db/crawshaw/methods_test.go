package crawshaw

import (
	"context"
	"testing"
	"time"
	
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"github.com/caasmo/restinpieces/db"
)

func createTestDB(t *testing.T) *Db {
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
	
	err = sqlitex.ExecScript(conn, `
		DROP TABLE IF EXISTS users;
		CREATE TABLE users (
			avatar TEXT DEFAULT '' NOT NULL,
			created TEXT DEFAULT '' NOT NULL,
			email TEXT DEFAULT '' NOT NULL,
			emailVisibility BOOLEAN DEFAULT FALSE NOT NULL,
			id TEXT PRIMARY KEY DEFAULT ('r'||lower(hex(randomblob(7)))) NOT NULL,
			name TEXT DEFAULT '' NOT NULL,
			password TEXT DEFAULT '' NOT NULL,
			tokenKey TEXT DEFAULT '' NOT NULL,
			updated TEXT DEFAULT '' NOT NULL,
			verified BOOLEAN DEFAULT FALSE NOT NULL
		);
		
		CREATE UNIQUE INDEX idx_tokenKey__pb_users_auth_ ON users(tokenKey);
		CREATE UNIQUE INDEX idx_email__pb_users_auth_ ON users(email) WHERE email != '';
		
		INSERT INTO users (id, email, name, password, created, updated, verified)
		VALUES ('test123', 'existing@test.com', 'Test User', 'hash123', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z', FALSE);
	`)
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
	testDB := createTestDB(t)
	defer testDB.Close()

	tests := []struct {
		name        string
		user        db.User
		wantErr     bool
		checkFields []string // Fields to verify in returned user
	}{
		{
			name: "valid user creation",
			user: db.User{
				Email:    "new@test.com",
				Password: "hashed_password_123",
				Name:     "New User",
				TokenKey: "token_key_123",
			},
			wantErr: false,
			checkFields: []string{"Email", "Name", "Password", "TokenKey"},
		},
		{
			name: "duplicate email",
			user: db.User{
				Email:    "existing@test.com", // Same email as test user created in createTestDB()
				Password: "hashed_password_123",
				Name:     "Duplicate User",
			},
			wantErr: true,
		},
		{
			name: "missing required fields",
			user: db.User{
				Email: "", // Empty email
				Name:  "Invalid User",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdUser, err := testDB.CreateUser(tt.user)
			
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				} else {
					t.Logf("expected error received: %v", err)
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
			if createdUser.Created == "" || createdUser.Updated == "" {
				t.Error("timestamps not set")
			}
			
			createdTime, err := time.Parse(time.RFC3339, createdUser.Created)
			if err != nil {
				t.Errorf("invalid created timestamp format: %v", err)
			}
			
			updatedTime, err := time.Parse(time.RFC3339, createdUser.Updated)
			if err != nil {
				t.Errorf("invalid updated timestamp format: %v", err)
			}

			// Verify timestamps are recent
			if time.Since(createdTime) > time.Minute {
				t.Error("created timestamp is too old")
			}
			if time.Since(updatedTime) > time.Minute {
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
	testDB := createTestDB(t)
	defer testDB.Close()
	
	tests := []struct {
		name        string
		email       string
		wantUser    *db.User
		wantErr     bool
	}{
		{
			name:  "existing user",
			email: "existing@test.com",
			wantUser: &db.User{
				ID:       "test123",
				Email:    "existing@test.com",
				Name:     "Test User",
				Password: "hash123",
				Created:  "2024-01-01T00:00:00Z",
				Updated:  "2024-01-01T00:00:00Z",
				Verified: false,
			},
			wantErr: false,
		},
		{
			name:    "non-existent user",
			email:   "nonexistent@test.com",
			wantUser: nil,
			wantErr: false,
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

