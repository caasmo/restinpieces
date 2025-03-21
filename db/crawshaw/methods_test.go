package crawshaw

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/migrations"
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
		name:      "users",
		schema:    migrations.UsersSchema,
		inserts:   []string{},
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

	// Create test user first
	testEmail := "test@example.com"
	testUser := &db.User{
		Email:    testEmail,
		Password: "testhash",
		Name:     "Test User",
		Verified: false,
	}

	createdUser, err := testDB.CreateUserWithPassword(*testUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name     string
		email    string
		wantUser *db.User
		wantErr  bool
	}{
		{
			name:     "existing user",
			email:    testEmail,
			wantUser: createdUser,
			wantErr:  false,
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


func TestCreateUserWithOauth2(t *testing.T) {
	testDB := setupDB(t)
	defer testDB.Close()

	// Base user data
	email := "test@example.com"
	baseUser := db.User{
		Email:    email,
		Name:     "Test User",
		Verified: true,
		Oauth2:   true,
	}

	// Test basic OAuth2 user creation
	t.Run("create oauth2 user", func(t *testing.T) {
		user := baseUser
		user.Avatar = "avatar1.jpg"

		createdUser, err := testDB.CreateUserWithOauth2(user)
		if err != nil {
			t.Fatalf("Failed to create oauth2 user: %v", err)
		}

		// Validate fields
		if createdUser.Email != user.Email {
			t.Errorf("Email mismatch: got %q, want %q", createdUser.Email, user.Email)
		}
		if createdUser.Avatar != user.Avatar {
			t.Errorf("Avatar mismatch: got %q, want %q", createdUser.Avatar, user.Avatar)
		}
		if !createdUser.Verified {
			t.Error("User should be verified")
		}
		if createdUser.Password != "" {
			t.Error("Password should be empty for OAuth2 users")
		}
		if !createdUser.Oauth2 {
			t.Error("Oauth2 flag should be true")
		}
	})

	// Test second OAuth2 provider with different avatar
	t.Run("add second oauth2 provider", func(t *testing.T) {
		user := baseUser
		user.Avatar = "avatar2.jpg"

		// Should succeed even though email exists, but avatar should stay original
		createdUser, err := testDB.CreateUserWithOauth2(user)
		if err != nil {
			t.Fatalf("Failed to add second oauth2 provider: %v", err)
		}

		if createdUser.Avatar != "avatar1.jpg" {
			t.Errorf("Avatar should remain as first value, got %q", createdUser.Avatar)
		}
	})

	// Test OAuth2 after password user
	t.Run("add oauth2 after password", func(t *testing.T) {
		// First create password user
		passwordUser := db.User{
			Email:    email,
			Password: "hashed_password",
			Verified: false,
			Oauth2:   false,
		}
		createdPwdUser, err := testDB.CreateUserWithPassword(passwordUser)
		if err != nil {
			t.Fatalf("Failed to create password user: %v", err)
		}

		// Add oauth2 auth
		oauth2User := baseUser
		createdOauth2User, err := testDB.CreateUserWithOauth2(oauth2User)
		if err != nil {
			t.Fatalf("Failed to add oauth2 auth: %v", err)
		}

		// Verify password remains and oauth2 is now true
		if createdOauth2User.Password != createdPwdUser.Password {
			t.Error("Password should be preserved from original user")
		}
		if !createdOauth2User.Oauth2 {
			t.Error("Oauth2 flag should be true after adding oauth2 auth")
		}
	})

	// Test adding password to OAuth2 user
	t.Run("add password to oauth2 user", func(t *testing.T) {
		// First create OAuth2 user
		oauthUser := db.User{
			Email:    "oauth2@test.com",
			Name:     "OAuth2 User",
			Avatar:   "avatar.jpg",
			Verified: true,
			Oauth2:   true,
		}
		_, err := testDB.CreateUserWithOauth2(oauthUser)
		if err != nil {
			t.Fatalf("Failed to create oauth2 user: %v", err)
		}

		// Add password auth
		passwordUser := db.User{
			Email:    "oauth2@test.com",
			Password: "new_hashed_password",
			Verified: false,
			Oauth2:   false,
		}
		createdUser, err := testDB.CreateUserWithPassword(passwordUser)
		if err != nil {
			t.Fatalf("Failed to add password auth: %v", err)
		}

		// Verify password is updated and oauth2 remains
		if createdUser.Password != passwordUser.Password {
			t.Error("Password should be updated")
		}
		if !createdUser.Oauth2 {
			t.Error("Oauth2 flag should remain true")
		}
		if !createdUser.Verified {
			t.Error("User should remain verified")
		}
		if createdUser.Avatar != oauthUser.Avatar {
			t.Errorf("Avatar should remain as oauth value, got %q", createdUser.Avatar)
		}
	})
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

