package zombiezen

import (
	"context"
	"fmt"
	"time"
	"github.com/caasmo/restinpieces/db"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

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
				created, err := db.TimeParse(stmt.GetText("created"))
				if err != nil {
					return fmt.Errorf("error parsing created time: %w", err)
				}

				updated, err := db.TimeParse(stmt.GetText("updated"))
				if err != nil {
					return fmt.Errorf("error parsing updated time: %w", err)
				}

				user = &db.User{
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
				}
				return nil
			},
			Args: []interface{}{email},
		})

	if err != nil {
		return nil, err
	}

	return user, nil
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
				created, err := db.TimeParse(stmt.GetText("created"))
				if err != nil {
					return fmt.Errorf("error parsing created time: %w", err)
				}

				updated, err := db.TimeParse(stmt.GetText("updated"))
				if err != nil {
					return fmt.Errorf("error parsing updated time: %w", err)
				}

				user = &db.User{
					ID:              stmt.GetText("id"),
					Email:           stmt.GetText("email"),
					Name:            stmt.GetText("name"),
					Password:        stmt.GetText("password"),
					Created:         created,
					Updated:         updated,
					Verified:        stmt.GetInt64("verified") != 0,
					Oauth2:          stmt.GetInt64("oauth2") != 0,
					EmailVisibility: stmt.GetInt64("emailVisibility") != 0,
				}
				return nil
			},
			Args: []interface{}{id},
		})

	if err != nil {
		return nil, err
	}

	return user, nil
}

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
			password = IIF(password = '', excluded.password, password),
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		RETURNING id, email, name, password, created, updated, verified`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				created, err := db.TimeParse(stmt.GetText("created"))
				if err != nil {
					return fmt.Errorf("error parsing created time: %w", err)
				}

				updated, err := db.TimeParse(stmt.GetText("updated"))
				if err != nil {
					return fmt.Errorf("error parsing updated time: %w", err)
				}

				createdUser = db.User{
					ID:       stmt.GetText("id"),
					Email:    stmt.GetText("email"),
					Password: stmt.GetText("password"),
					Created:  created,
					Updated:  updated,
					Verified: stmt.GetInt64("verified") != 0,
				}
				return nil
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

	return &createdUser, err
}

func (d *Db) CreateUserWithOauth2(user db.User) (*db.User, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, err
	}
	defer d.pool.Put(conn)

	now := time.Now().UTC().Format(time.RFC3339)

	var createdUser db.User
	err = sqlitex.Execute(conn,
		`INSERT INTO users (name, password, verified, oauth2, avatar, email, emailVisibility) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET
			oauth2 = true,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		RETURNING id, email, name, password, created, updated, verified`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				created, err := db.TimeParse(stmt.GetText("created"))
				if err != nil {
					return fmt.Errorf("error parsing created time: %w", err)
				}

				updated, err := db.TimeParse(stmt.GetText("updated"))
				if err != nil {
					return fmt.Errorf("error parsing updated time: %w", err)
				}

				createdUser = db.User{
					ID:       stmt.GetText("id"),
					Email:    stmt.GetText("email"),
					Password: stmt.GetText("password"),
					Created:  created,
					Updated:  updated,
					Verified: stmt.GetInt64("verified") != 0,
				}
				return nil
			},
			Args: []interface{}{
				user.Name,            // 1. name
				"",                   // 2. password
				user.Verified,        // 3. verified
				true,                 // 4. oauth2
				user.Avatar,          // 5. avatar
				user.Email,           // 6. email
				user.EmailVisibility, // 7. emailVisibility
			},
		})

	return &createdUser, err
}

func (d *Db) VerifyEmail(userId string) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return err
	}
	defer d.pool.Put(conn)

	return sqlitex.Execute(conn,
		`UPDATE users 
		SET verified = true,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		WHERE id = ?`,
		&sqlitex.ExecOptions{
			Args: []interface{}{userId},
		})
}

func (d *Db) UpdatePassword(userId string, newPassword string) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	defer d.pool.Put(conn)

	// Validate password length before update
	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	// Update password and timestamp
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

	// Update email and timestamp
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

