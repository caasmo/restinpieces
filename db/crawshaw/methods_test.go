package crawshaw

import (
	"context"
	"testing"
	"time"
	
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"github.com/caasmo/restinpieces/crypto"
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

