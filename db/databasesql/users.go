package databasesql

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/caasmo/restinpieces/db"
)

// rowScanner is the Scan method shared by *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// userColumns is the column list every users query selects.
const userColumns = `id, name, password, verified, oauth2, avatar, email, emailVisibility, created, updated`

// newUserFromRow creates a User struct from a scanned database row.
// BOOLEAN columns arrive as int64; they are converted with != 0.
func newUserFromRow(row rowScanner) (*db.User, error) {
	var (
		u               db.User
		verified        int64
		oauth2          int64
		emailVisibility int64
		createdStr      string
		updatedStr      string
	)
	err := row.Scan(
		&u.ID, &u.Name, &u.Password, &verified, &oauth2,
		&u.Avatar, &u.Email, &emailVisibility, &createdStr, &updatedStr,
	)
	if err != nil {
		return nil, err
	}

	u.Verified = verified != 0
	u.Oauth2 = oauth2 != 0
	u.EmailVisibility = emailVisibility != 0

	created, err := db.TimeParse(createdStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing created time: %w", err)
	}
	u.Created = created

	updated, err := db.TimeParse(updatedStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing updated time: %w", err)
	}
	u.Updated = updated

	return &u, nil
}

// GetUserByEmail retrieves a user by email address.
// Returns:
// - *db.User: User record if found, nil if no matching record exists
// - returned time fields are in UTC, RFC3339
// - error: Only returned for database errors, nil on successful query (even if no results)
// Note: A nil user with nil error indicates no matching record was found
func (d *Db) GetUserByEmail(email string) (*db.User, error) {
	user, err := newUserFromRow(d.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE email = ? LIMIT 1`, email))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserById retrieves a user by ID.
func (d *Db) GetUserById(id string) (*db.User, error) {
	user, err := newUserFromRow(d.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE id = ? LIMIT 1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// CreateUserWithPassword inserts a new user with a password.
//
// # Security: Password Protection on Conflict
//
// On email conflict, this method intentionally does NOT update the password.
// Only the updated timestamp is touched.
//
// This prevents account takeover via the unauthenticated registration endpoint:
// an attacker who knows a valid email — whether the account was created with
// a password or OAuth2 — cannot overwrite the real user's credentials.
// OAuth2 users have password='' in the DB; without this protection the IIF
// trick used previously (IIF(password='', excluded.password, password)) would
// still allow overwriting their empty password with an attacker-chosen one.
//
// Changing a password is an authenticated action and belongs in a dedicated
// settings endpoint, not here.
//
// The caller (RegisterWithPasswordHandler) always returns the same response
// regardless of conflict, so no information about email existence is leaked.
func (d *Db) CreateUserWithPassword(user db.User) (*db.User, error) {
	createdUser, err := newUserFromRow(d.db.QueryRow(
		`INSERT INTO users (name, password, verified, oauth2, avatar, email, emailVisibility) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		RETURNING `+userColumns,
		user.Name, user.Password, user.Verified, false, user.Avatar, user.Email, user.EmailVisibility))
	if err != nil {
		return nil, err
	}
	return createdUser, nil
}

// CreateUserWithOauth2 inserts a new user authenticated via OAuth2.
//
// Strict INSERT — no ON CONFLICT update. The handler upstream has already
// established via GetUserByEmail that the email is either absent or belongs
// to an existing OAuth2 user. Mutating an existing row here is never correct;
// account linking belongs in the authenticated /link-oauth2 endpoint.
//
// On email conflict (narrow race between GetUserByEmail and this insert)
// the database layer returns an error which the caller surfaces as a generic
// DB error.
func (d *Db) CreateUserWithOauth2(user db.User) (*db.User, error) {
	createdUser, err := newUserFromRow(d.db.QueryRow(
		`INSERT INTO users (name, password, verified, oauth2, avatar, email, emailVisibility)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING `+userColumns,
		user.Name, "", true, true, user.Avatar, user.Email, user.EmailVisibility))
	if err != nil {
		return nil, err
	}
	return createdUser, nil
}

// UpdateVerified marks the user with the given email as verified. Idempotent:
// if already verified, it just updates the timestamp and returns the row.
// If the email does not exist (token issued for a non-existent account),
// returns an empty user with no error, matching the previous driver's
// behavior.
func (d *Db) UpdateVerified(email string) (*db.User, error) {
	user, err := newUserFromRow(d.db.QueryRow(
		`UPDATE users 
        SET verified = true,
            updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        WHERE email = ?
        RETURNING `+userColumns, email))
	if errors.Is(err, sql.ErrNoRows) {
		return &db.User{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to verify email: %w", err)
	}
	return user, nil
}

// UpdatePassword updates the password of the user with the given ID.
func (d *Db) UpdatePassword(userId string, newPassword string) error {
	_, err := d.db.Exec(
		`UPDATE users 
		SET password = ?,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		WHERE id = ?`,
		newPassword, userId)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

// UpdateEmail updates the email of the user with the given ID.
func (d *Db) UpdateEmail(userId string, newEmail string) error {
	_, err := d.db.Exec(
		`UPDATE users 
		SET email = ?,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		WHERE id = ?`,
		newEmail, userId)
	if err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}
	return nil
}
