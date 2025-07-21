package zombiezen

import (
	"context"
	"io/fs"
	"testing"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newTestUserDB creates a new in-memory SQLite database and applies the users schema.
func newTestUserDB(t *testing.T) *Db {
	t.Helper()

	pool, err := sqlitex.NewPool("file::memory:", sqlitex.PoolOptions{
		PoolSize: 1,
	})
	if err != nil {
		t.Fatalf("failed to create db pool: %v", err)
	}

	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("failed to close db pool: %v", err)
		}
	})

	conn, err := pool.Take(context.Background())
	if err != nil {
		t.Fatalf("failed to get db connection: %v", err)
	}
	defer pool.Put(conn)

	schemaFS := migrations.Schema()
	sqlBytes, err := fs.ReadFile(schemaFS, "app/users.sql")
	if err != nil {
		t.Fatalf("Failed to read app/users.sql: %v", err)
	}

	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		t.Fatalf("Failed to execute app/users.sql: %v", err)
	}

	db, err := New(pool)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	return db
}

func TestUserLifecycle(t *testing.T) {
	testDB := newTestUserDB(t)
	var userPassword, userOauth *db.User
	var err error

	t.Run("CreateWithPassword", func(t *testing.T) {
		userPassword, err = testDB.CreateUserWithPassword(db.User{
			Name:     "Test User",
			Email:    "test@example.com",
			Password: "password123",
		})
		if err != nil {
			t.Fatalf("CreateUserWithPassword failed: %v", err)
		}
		if userPassword.ID == "" {
			t.Fatal("expected user to have an ID")
		}
		if userPassword.Password != "password123" {
			t.Errorf("expected password to be 'password123', got %q", userPassword.Password)
		}
		if userPassword.Oauth2 {
			t.Error("expected Oauth2 to be false")
		}
	})

	t.Run("CreateWithOauth2", func(t *testing.T) {
		userOauth, err = testDB.CreateUserWithOauth2(db.User{
			Name:  "Oauth User",
			Email: "oauth@example.com",
		})
		if err != nil {
			t.Fatalf("CreateUserWithOauth2 failed: %v", err)
		}
		if userOauth.ID == "" {
			t.Fatal("expected oauth user to have an ID")
		}
		if userOauth.Password != "" {
			t.Errorf("expected password to be empty, got %q", userOauth.Password)
		}
		if !userOauth.Oauth2 {
			t.Error("expected Oauth2 to be true")
		}
	})

	t.Run("GetByEmail", func(t *testing.T) {
		fetchedUser, err := testDB.GetUserByEmail("test@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}
		if fetchedUser == nil {
			t.Fatal("expected to fetch a user, but got nil")
		}
		if fetchedUser.ID != userPassword.ID {
			t.Errorf("expected user ID %q, got %q", userPassword.ID, fetchedUser.ID)
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		fetchedUser, err := testDB.GetUserById(userPassword.ID)
		if err != nil {
			t.Fatalf("GetUserById failed: %v", err)
		}
		if fetchedUser == nil {
			t.Fatal("expected to fetch a user, but got nil")
		}
		if fetchedUser.Email != "test@example.com" {
			t.Errorf("expected user email 'test@example.com', got %q", fetchedUser.Email)
		}
	})

	t.Run("UpdatePassword", func(t *testing.T) {
		err := testDB.UpdatePassword(userPassword.ID, "newpassword")
		if err != nil {
			t.Fatalf("UpdatePassword failed: %v", err)
		}
		fetchedUser, _ := testDB.GetUserById(userPassword.ID)
		if fetchedUser.Password != "newpassword" {
			t.Errorf("expected password to be 'newpassword', got %q", fetchedUser.Password)
		}
		userPassword = fetchedUser // Update for subsequent tests
	})

	t.Run("UpdateEmail", func(t *testing.T) {
		err := testDB.UpdateEmail(userPassword.ID, "new-email@example.com")
		if err != nil {
			t.Fatalf("UpdateEmail failed: %v", err)
		}
		fetchedUser, _ := testDB.GetUserByEmail("new-email@example.com")
		if fetchedUser == nil {
			t.Fatal("failed to fetch user by new email")
		}
		if fetchedUser.ID != userPassword.ID {
			t.Errorf("fetched user by new email has wrong ID")
		}
	})

	t.Run("VerifyEmail", func(t *testing.T) {
		err := testDB.VerifyEmail(userPassword.ID)
		if err != nil {
			t.Fatalf("VerifyEmail failed: %v", err)
		}
		fetchedUser, _ := testDB.GetUserById(userPassword.ID)
		if !fetchedUser.Verified {
			t.Error("expected user to be verified, but they are not")
		}
	})
}

func TestUser_EdgeCases(t *testing.T) {
	testDB := newTestUserDB(t)

	t.Run("GetNonExistentUser", func(t *testing.T) {
		user, err := testDB.GetUserByEmail("no-such-user@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail for non-existent user returned error: %v", err)
		}
		if user != nil {
			t.Fatal("expected nil when getting non-existent user by email")
		}

		user, err = testDB.GetUserById("non-existent-id")
		if err != nil {
			t.Fatalf("GetUserById for non-existent user returned error: %v", err)
		}
		if user != nil {
			t.Fatal("expected nil when getting non-existent user by id")
		}
	})

	t.Run("CreateConflict", func(t *testing.T) {
		// 1. Create a user via OAuth, which results in an empty password
		_, err := testDB.CreateUserWithOauth2(db.User{
			Name:  "Conflict User",
			Email: "conflict@example.com",
		})
		if err != nil {
			t.Fatalf("OAuth user creation failed: %v", err)
		}

		// 2. Now, call CreateUserWithPassword to set the password for the first time
		userWithPassword, err := testDB.CreateUserWithPassword(db.User{
			Email:    "conflict@example.com",
			Password: "password1",
		})
		if err != nil {
			t.Fatalf("Setting initial password failed: %v", err)
		}
		if userWithPassword.Password != "password1" {
			t.Errorf("expected password to be set to 'password1', got %q", userWithPassword.Password)
		}

		// 3. Attempt to change the password using the same function
		userWithUnchangedPwd, err := testDB.CreateUserWithPassword(db.User{
			Email:    "conflict@example.com",
			Password: "password2", // This should be ignored
		})
		if err != nil {
			t.Fatalf("Attempting to change password failed: %v", err)
		}
		// Verify the password was NOT changed
		if userWithUnchangedPwd.Password != "password1" {
			t.Errorf("expected password to remain 'password1', but got %q", userWithUnchangedPwd.Password)
		}
	})

	t.Run("UpdateNonExistentUser", func(t *testing.T) {
		// These should be no-ops and not return errors
		err := testDB.UpdatePassword("non-existent-id", "new-password")
		if err != nil {
			t.Errorf("UpdatePassword for non-existent user returned an error: %v", err)
		}

		err = testDB.UpdateEmail("non-existent-id", "new@example.com")
		if err != nil {
			t.Errorf("UpdateEmail for non-existent user returned an error: %v", err)
		}
	})
}
