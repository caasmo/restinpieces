package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"strings"
)

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

			// Get the date strings
			createdStr := stmt.GetText("created")
			updatedStr := stmt.GetText("updated")

			created, err := db.TimeParse(createdStr)
			if err != nil {
				return fmt.Errorf("error parsing created time: %w", err)
			}

			updated, err := db.TimeParse(updatedStr)
			if err != nil {
				return fmt.Errorf("error parsing updated time: %w", err)
			}

			user = &db.User{
				ID:           stmt.GetText("id"),
				Email:        stmt.GetText("email"),
				Name:         stmt.GetText("name"),
				Password:     stmt.GetText("password"),
				Created:      created,
				Updated:      updated,
				Verified:     stmt.GetInt64("verified") != 0,
				ExternalAuth:    stmt.GetText("externalAuth"),
				EmailVisibility: stmt.GetInt64("emailVisibility") != 0,
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

			// Get the date strings
			createdStr := stmt.GetText("created")
			updatedStr := stmt.GetText("updated")

			created, err := db.TimeParse(createdStr)
			if err != nil {
				return fmt.Errorf("error parsing created time: %w", err)
			}

			updated, err := db.TimeParse(updatedStr)
			if err != nil {
				return fmt.Errorf("error parsing updated time: %w", err)
			}

			user = &db.User{
				ID:       stmt.GetText("id"),
				Email:    stmt.GetText("email"),
				Name:     stmt.GetText("name"),
				Password: stmt.GetText("password"),
				Created:  created,
				Updated:  updated,
				Verified: stmt.GetInt64("verified") != 0,
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

// CreateUser inserts a new user with all fields from users.sql schema
func (d *Db) CreateUser(user db.User) (*db.User, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var createdUser *db.User
	err := sqlitex.Exec(conn,
		`INSERT INTO users (name, password, verified, externalAuth, avatar, email, emailVisibility) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, name, password, verified, externalAuth, avatar, email, emailVisibility, created, updated`,
		func(stmt *sqlite.Stmt) error {
			created, err := db.TimeParse(stmt.GetText("created"))
			if err != nil {
				return fmt.Errorf("error parsing created time: %w", err)
			}

			updated, err := db.TimeParse(stmt.GetText("updated"))
			if err != nil {
				return fmt.Errorf("error parsing updated time: %w", err)
			}

			createdUser = &db.User{
				ID:              stmt.GetText("id"),
				Name:           stmt.GetText("name"),
				Password:       stmt.GetText("password"),
				Verified:       stmt.GetInt64("verified") != 0,
				ExternalAuth:   stmt.GetText("externalAuth"),
				Avatar:         stmt.GetText("avatar"),
				Email:          stmt.GetText("email"),
				EmailVisibility: stmt.GetInt64("emailVisibility") != 0,
				Created:        created,
				Updated:        updated,
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
