package crawshaw_test

import (
	"context"
	"testing"
	"time"
	
	"crawshaw.io/sqlite/sqlitex"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/crawshaw"
)

func createTestDB(t *testing.T) *crawshaw.Db {
	t.Helper()
	
	pool, err := sqlitex.Open("file:testdb?mode=memory&cache=shared", 0, 4)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	
	conn := pool.Get(context.TODO())
	defer pool.Put(conn)
	
	err = sqlitex.ExecScript(conn, `
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			password TEXT NOT NULL,
			created TEXT NOT NULL,
			updated TEXT NOT NULL,
			verified BOOLEAN NOT NULL DEFAULT FALSE,
			token_key TEXT
		);
		INSERT INTO users (id, email, name, password, created, updated)
		VALUES ('test123', 'existing@test.com', 'Test User', 'hash123', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z');
	`)
	if err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}
	
	// Create new DB instance properly using exported constructor
	db, err := crawshaw.New("file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to create db instance: %v", err)
	}
	return db
}

func TestGetUserByEmail(t *testing.T) {
	testDB := createTestDB(t)
	
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
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := testDB.GetUserByEmail(tt.email)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserByEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if tt.wantUser != nil {
				if user.ID != tt.wantUser.ID ||
					user.Email != tt.wantUser.Email ||
					user.Name != tt.wantUser.Name ||
					user.Password != tt.wantUser.Password ||
					user.Created != tt.wantUser.Created ||
					user.Updated != tt.wantUser.Updated ||
					user.Verified != tt.wantUser.Verified {
					t.Errorf("GetUserByEmail() = %+v, want %+v", user, tt.wantUser)
				}
			}
		})
	}
}

func TestCreateUser(t *testing.T) {
	testDB := createTestDB(t)
	
	tests := []struct {
		name        string
		email       string
		password    string
		username    string
		wantErr     bool
	}{
		{
			name:     "valid new user",
			email:    "newuser@test.com",
			password: "securepassword123",
			username: "New User",
			wantErr:  false,
		},
		{
			name:     "duplicate email",
			email:    "existing@test.com",
			password: "password123",
			username: "Duplicate User",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := testDB.CreateUser(tt.email, tt.password, tt.username)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				// Validate returned user data
				if user.Email != tt.email {
					t.Errorf("CreateUser() email = %v, want %v", user.Email, tt.email)
				}
				if user.Name != tt.username {
					t.Errorf("CreateUser() name = %v, want %v", user.Name, tt.username)
				}
				if !crypto.CheckPassword(tt.password, user.Password) {
					t.Error("CreateUser() password hash validation failed")
				}
				
				// Validate timestamps
				if _, err := time.Parse(time.RFC3339, user.Created); err != nil {
					t.Errorf("Invalid created timestamp format: %v", user.Created)
				}
				if _, err := time.Parse(time.RFC3339, user.Updated); err != nil {
					t.Errorf("Invalid updated timestamp format: %v", user.Updated)
				}
				
				// Verify the user exists in DB
				dbUser, err := testDB.GetUserByEmail(tt.email)
				if err != nil {
					t.Errorf("Failed to retrieve created user: %v", err)
				}
				if dbUser.ID != user.ID {
					t.Errorf("Retrieved user ID mismatch: want %v, got %v", user.ID, dbUser.ID)
				}
			}
		})
	}
}
