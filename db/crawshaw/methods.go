package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"strings"
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
		`SELECT id, name, password, verified, externalAuth, avatar, email, emailVisibility, created, updated
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
func validateQueueJob(job queue.QueueJob) error {
	var missingFields []string
	if job.JobType == "" {
		missingFields = append(missingFields, "JobType")
	}
	if len(job.Payload) == 0 {
		missingFields = append(missingFields, "Payload")
	}
	if job.MaxAttempts < 1 {
		missingFields = append(missingFields, "MaxAttempts must be ≥1")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("%w: %s", db.ErrMissingFields, strings.Join(missingFields, ", "))
	}
	return nil
}

// GetUserById retrieves a user by their unique ID.
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

func (d *Db) InsertQueueJob(job queue.QueueJob) error {
	if err := validateQueueJob(job); err != nil {
		return err
	}

	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	err := sqlitex.Exec(conn, `INSERT INTO job_queue 
		(job_type, payload, attempts, max_attempts) 
		VALUES (?, ?, ?, ?)`,
		nil,                 // No results needed for INSERT
		job.JobType,         // 1. job_type
		string(job.Payload), // 2. payload
		job.Attempts,        // 4. attempts
		job.MaxAttempts,     // 5. max_attempts
	)

	if err != nil {
		if sqliteErr, ok := err.(sqlite.Error); ok {
			if sqliteErr.Code == sqlite.SQLITE_CONSTRAINT_UNIQUE {
				return db.ErrConstraintUnique
			}
		}
		return fmt.Errorf("queue insert failed: %w", err)
	}
	return nil
}

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

//So if these happen concurrently:
//- Password registration updates password-specific fields
//- OAuth2 registration updates OAuth-specific fields
//The resulting user will have both authentication methods properly set up without either one completely overwriting the other.
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
		true,                 // 3. verified
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

// CreateUser inserts a new user with all fields from users.sql schema
// TODO updated has to be explicite set in the struct, DEFAULT only works on create.
// Document EmailVisibility
// TODO move TO CreateUserWithPassword and CreateUserWithOauthj2
// TODO method will not return error unique, let the aplication handle
// if password are diferent say user already exist user 
// if password match and oauth2 user has already oauth2 auth and does not need email validation
// if password match and no aoutj2, record created
//do not forget to update on conflich
func (d *Db) CreateUser(user db.User) (*db.User, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

//ON CONFLICT(email) DO UPDATE SET
//      password = CASE 
//	          WHEN password = '' THEN excluded.password 
//			          ELSE password 
//					        END,

	var createdUser *db.User
	err := sqlitex.Exec(conn,
		`INSERT INTO users (name, password, verified, externalAuth, avatar, email, emailVisibility) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, name, password, verified, externalAuth, avatar, email, emailVisibility, created, updated`,
		func(stmt *sqlite.Stmt) error {

			var err error
			createdUser, err = newUserFromStmt(stmt)
			if err != nil {
				return err
			}
			return nil
		},
		user.Name,            // 1. name
		user.Password,        // 2. password
		user.Verified,        // 3. verified
		user.ExternalAuth,    // 4. externalAuth
		user.Avatar,          // 5. avatar
		user.Email,           // 6. email
		user.EmailVisibility, // 7. emailVisibility
	)

	if err != nil {
		if sqliteErr, ok := err.(sqlite.Error); ok {
			if sqliteErr.Code == sqlite.SQLITE_CONSTRAINT_UNIQUE {
				return nil, db.ErrConstraintUnique
			}
		}
		return nil, err
	}

	return createdUser, nil
}
