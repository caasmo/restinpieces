package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"github.com/caasmo/restinpieces/db"
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
// - returned time Fields are in UTC, RFC3339
// - error: Only returned for database errors, nil on successful query (even if no results)
// Note: A nil user with nil error indicates no matching record was found
func (d *Db) GetUserByEmail(email string) (*db.User, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var user *db.User // Will remain nil if no rows found
	err := sqlitex.Exec(conn,
		`SELECT id, name, password, verified, oauth2, avatar, email, emailVisibility, created, updated
		FROM users WHERE email = ? LIMIT 1`,
		func(stmt *sqlite.Stmt) error {

			var err error
			user, err = newUserFromStmt(stmt)
			if err != nil {
				return err
			}
			return nil
		}, email)

	if err != nil {
		return nil, err
	}

	return user, nil
}

// validateUserFields checks that required user fields are present
// Returns:
// - *db.User: User record if found, nil if no matching record exists
// - returned time Fields are in UTC, RFC3339
// - error: Only returned for database errors, nil on successful query (even if no results)
// Note: A nil user with nil error indicates no matching record was found
func (d *Db) GetUserById(id string) (*db.User, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var user *db.User // Will remain nil if no rows found
	err := sqlitex.Exec(conn,
		`SELECT id, name, password, verified, externalAuth, avatar, email, emailVisibility, created, updated
		FROM users WHERE id = ? LIMIT 1`,
		func(stmt *sqlite.Stmt) error {

			var err error
			user, err = newUserFromStmt(stmt)
			if err != nil {
				return err
			}
			return nil
		}, id)

	if err != nil {
		return nil, err
	}

	return user, nil
}


// writing os two consecutive writes with two different password will succeed but the password will be not written.
// its responsability of the caller to check if interested.
func (d *Db) CreateUserWithPassword(user db.User) (*db.User, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var createdUser *db.User
	err := sqlitex.Exec(conn,
		`INSERT INTO users (name, password, verified, oauth2, avatar, email, emailVisibility) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET 
			password = IIF(password = '', excluded.password, password),
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		RETURNING id, name, password, verified, oauth2, avatar, email, emailVisibility, created, updated`,
		func(stmt *sqlite.Stmt) error {
			var err error
			createdUser, err = newUserFromStmt(stmt)
			return err
		},
		user.Name,            // 1. name
		user.Password,        // 2. password
		user.Verified,        // 3. verified
		false,                // 4. oauth2
		user.Avatar,          // 5. avatar
		user.Email,           // 6. email
		user.EmailVisibility, // 7. emailVisibility
	)

	if err != nil {
		return nil, err
	}

	return createdUser, nil
}

// So if these happen concurrently:
// - Password registration updates password-specific fields
// - OAuth2 registration updates OAuth-specific fields
// The resulting user will have both authentication methods properly set up without either one completely overwriting the other.
func (d *Db) CreateUserWithOauth2(user db.User) (*db.User, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var createdUser *db.User
	err := sqlitex.Exec(conn,
		`INSERT INTO users (name, password, verified, oauth2, avatar, email, emailVisibility) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET 
			oauth2 = true,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		RETURNING id, name, password, verified, oauth2, avatar, email, emailVisibility, created, updated`,
		func(stmt *sqlite.Stmt) error {
			var err error
			createdUser, err = newUserFromStmt(stmt)
			return err
		},
		user.Name,            // 1. name
		"",                   // 2. password
		user.Verified,        // 3. verified
		true,                 // 4. oauth2
		user.Avatar,          // 5. avatar
		user.Email,           // 6. email
		user.EmailVisibility, // 7. emailVisibility
	)

	if err != nil {
		return nil, err
	}

	return createdUser, nil
}
