package zombiezen

import (
	"context"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// newUserFromStmt creates a User struct from a SQLite statement
func newUserFromStmt(stmt *sqlite.Stmt) (*db.User, error) {
	created, err := db.TimeParse(stmt.GetText("created"))
	if err != nil {
		return nil, fmt.Errorf("error parsing created time: %w", err)
	}

	updated, err := db.TimeParse(stmt.GetText("updated"))
	if err != nil {
		return nil, fmt.Errorf("error parsing updated time: %w", err)
	}

	return &db.User{
		ID:              stmt.GetText("id"),
		Name:            stmt.GetText("name"),
		Password:        stmt.GetText("password"),
		Verified:        stmt.GetInt64("verified") != 0,
		Oauth2:          stmt.GetInt64("oauth2") != 0,
		Avatar:          stmt.GetText("avatar"),
		Email:           stmt.GetText("email"),
		EmailVisibility: stmt.GetInt64("emailVisibility") != 0,
		Created:         created,
		Updated:         updated,
	}, nil
}

// GetUserByEmail retrieves a user by email address.
// Returns:
// - *db.User: User record if found, nil if no matching record exists
// - returned time fields are in UTC, RFC3339
// - error: Only returned for database errors, nil on successful query (even if no results)
// Note: A nil user with nil error indicates no matching record was found
func (d *Db) GetUserByEmail(email string) (*db.User, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, err
	}
	defer d.pool.Put(conn)

	var user *db.User // Will remain nil if no rows found
	err = sqlitex.Execute(conn,
		`SELECT id, name, password, verified, oauth2, avatar, email, emailVisibility, created, updated
		FROM users WHERE email = ? LIMIT 1`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				var err error
				user, err = newUserFromStmt(stmt)
				return err
			},
			Args: []interface{}{email},
		})

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (d *Db) VerifyEmail(userId string) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return err
	}
	defer d.pool.Put(conn)

	err = sqlitex.Execute(conn,
		`UPDATE users 
		SET verified = true,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		WHERE id = ?`,
		&sqlitex.ExecOptions{
			Args: []interface{}{userId},
		})
	if err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}
	return nil
}

func (d *Db) GetUserById(id string) (*db.User, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, err
	}
	defer d.pool.Put(conn)

	var user *db.User
	err = sqlitex.Execute(conn,
		`SELECT id, name, password, verified, oauth2, avatar, email, emailVisibility, created, updated
		FROM users WHERE id = ? LIMIT 1`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				var err error
				user, err = newUserFromStmt(stmt)
				return err
			},
			Args: []interface{}{id},
		})

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
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, err
	}
	defer d.pool.Put(conn)

	var createdUser db.User
	err = sqlitex.Execute(conn,
		`INSERT INTO users (name, password, verified, oauth2, avatar, email, emailVisibility) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		RETURNING id, name, password, verified, oauth2, avatar, email, emailVisibility, created, updated`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				tempUser, err := newUserFromStmt(stmt)
				if err == nil && tempUser != nil {
					createdUser = *tempUser
				}
				return err
			},
			Args: []interface{}{
				user.Name,            // 1. name
				user.Password,        // 2. password
				user.Verified,        // 3. verified
				false,                // 4. oauth2
				user.Avatar,          // 5. avatar
				user.Email,           // 6. email
				user.EmailVisibility, // 7. emailVisibility
			},
		})

	if err != nil {
		return nil, err
	}
	return &createdUser, nil
}

func (d *Db) CreateUserWithOauth2(user db.User) (*db.User, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, err
	}
	defer d.pool.Put(conn)

	var createdUser db.User
	err = sqlitex.Execute(conn,
		`INSERT INTO users (name, password, verified, oauth2, avatar, email, emailVisibility) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET
			oauth2 = true,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		RETURNING id, name, password, verified, oauth2, avatar, email, emailVisibility, created, updated`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				tempUser, err := newUserFromStmt(stmt)
				if err == nil && tempUser != nil {
					createdUser = *tempUser
				}
				return err
			},
			Args: []interface{}{
				user.Name,            // 1. name
				"",                   // 2. password — OAuth2 users have no password
				user.Verified,        // 3. verified
				true,                 // 4. oauth2
				user.Avatar,          // 5. avatar
				user.Email,           // 6. email
				user.EmailVisibility, // 7. emailVisibility
			},
		})

	if err != nil {
		return nil, err
	}
	return &createdUser, nil
}

func (d *Db) UpdatePassword(userId string, newPassword string) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	defer d.pool.Put(conn)

	err = sqlitex.Execute(conn,
		`UPDATE users 
		SET password = ?,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		WHERE id = ?`,
		&sqlitex.ExecOptions{
			Args: []interface{}{newPassword, userId},
		})
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

func (d *Db) UpdateEmail(userId string, newEmail string) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	defer d.pool.Put(conn)

	err = sqlitex.Execute(conn,
		`UPDATE users 
		SET email = ?,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		WHERE id = ?`,
		&sqlitex.ExecOptions{
			Args: []interface{}{newEmail, userId},
		})
	if err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}

	return nil
}
