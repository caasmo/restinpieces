//go:build ignore

// Command adduser creates or updates a password user directly in the
// application database, so a working login can be handed out without the
// registration and email verification flow.
//
// The database is data/app.db, the conventional path in the project home
// (doc/layout-best-practices.md). The email and password are normalised
// and validated with the same rules as the registration endpoint, the
// password is hashed with the same argon2id the app verifies with, and
// the account is marked verified — so the first login works.
//
// Usage (from repo root):
//
//	go run scripts/adduser.go you@example.com '<password>'
//
// The password argument lands in the shell history and the process list;
// clear the history afterwards where that matters.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/crypto"
)

// dbPath is hardcoded: the script runs in the project home, where the
// database lives at the conventional path.
const dbPath = "data/app.db"

func main() {
	// Step 1: read the two arguments.
	if len(os.Args) != 3 {
		usage()
	}
	email := strings.ToLower(strings.TrimSpace(os.Args[1]))
	password := strings.TrimSpace(os.Args[2])

	// Step 2: validate with the app's own rules, so the stored record is
	// one the registration endpoint would have accepted.
	validator := core.NewValidator()

	err := validator.Email(email)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "invalid email: %v\n", err)
		os.Exit(1)
	}

	err = validator.Password(password)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "invalid password: %v\n", err)
		os.Exit(1)
	}

	// Step 3: hash the password with the same argon2id the app verifies
	// with.
	passwordHash := crypto.HashPassword(password)

	// Step 4: open the application database. The file must already exist;
	// opening a missing path would create an empty database in the wrong
	// directory and hide the mistake behind a table error.
	_, statErr := os.Stat(dbPath)
	if statErr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "cannot access %q: %v (run in the project home)\n", dbPath, statErr)
		os.Exit(1)
	}

	pool, err := restinpieces.NewModerncPool(dbPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "open db %q: %v\n", dbPath, err)
		os.Exit(1)
	}
	defer func() {
		_ = pool.Close()
	}()

	// Step 5: upsert the user by email, marked verified.
	user, err := upsertUser(pool, email, passwordHash)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "upsert user: %v\n", err)
		os.Exit(1)
	}

	// Step 6: report.
	_, _ = fmt.Printf("user %s <%s> ready for password login\n", user.ID, user.Email)
}

// usage prints how to call the script and exits.
func usage() {
	_, _ = fmt.Fprintf(os.Stderr, "usage: %s <email> <password>\n", os.Args[0])
	os.Exit(1)
}

// userRow is the part of a users row the script reports.
type userRow struct {
	ID    string
	Email string
}

// upsertUser inserts a user, or replaces the password and verification of
// the existing row with the same email.
//
// The app's own CreateUserWithPassword keeps an existing password on
// conflict because its caller is the unauthenticated registration
// endpoint; here the operator is inside the trust boundary and asked for
// a working login, so the password is replaced and the account is marked
// verified.
func upsertUser(pool *sql.DB, email, passwordHash string) (userRow, error) {
	var user userRow

	err := pool.QueryRow(
		`INSERT INTO users (email, password, verified)
		VALUES (?, ?, TRUE)
		ON CONFLICT(email) DO UPDATE SET
			password = excluded.password,
			verified = excluded.verified,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		RETURNING id, email`,
		email, passwordHash).Scan(&user.ID, &user.Email)
	if err != nil {
		return userRow{}, fmt.Errorf("failed to upsert user: %w", err)
	}

	return user, nil
}
