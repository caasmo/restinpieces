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
		`SELECT id, email, name, password, created, updated, verified, tokenKey 
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
				ID:       stmt.GetText("id"),
				Email:    stmt.GetText("email"),
				Name:     stmt.GetText("name"),
				Password: stmt.GetText("password"),
				Created:  created,
				Updated:  updated,
				Verified: stmt.GetInt64("verified") != 0,
				TokenKey: stmt.GetText("tokenKey"),
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

func validateUserFields(user db.User) error {
	var missingFields []string
	if user.Email == "" {
		missingFields = append(missingFields, "Email")
	}
	if user.Password == "" {
		missingFields = append(missingFields, "Password")
	}
	if user.TokenKey == "" {
		missingFields = append(missingFields, "TokenKey")
	}

	if len(missingFields) > 0 {
		return fmt.Errorf("%w: %s", db.ErrMissingFields, strings.Join(missingFields, ", "))
	}
	return nil
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

// CreateUser inserts a new user with RFC3339 formatted UTC timestamps.
// Example timestamp: "2024-03-07T15:04:05Z"
// User struct should contain at minimum: Email, Password (pre-hashed), and Name
// time fields are ignores in favor of default sqlite values
func (d *Db) CreateUser(user db.User) (*db.User, error) {
	// Validate required fields
	if err := validateUserFields(user); err != nil {
		return nil, err
	}

	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var createdUser *db.User
	err := sqlitex.Exec(conn,
		`INSERT INTO users (email, password, name, tokenKey) 
		VALUES (?, ?, ?, ?)
		RETURNING id, email, name, password, created, updated, verified, tokenKey`,
		func(stmt *sqlite.Stmt) error {
			// Get and parse timestamps from database
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

			createdUser = &db.User{
				ID:       stmt.GetText("id"),
				Email:    stmt.GetText("email"),
				Name:     stmt.GetText("name"),
				Password: stmt.GetText("password"),
				Created:  created,
				Updated:  updated,
				Verified: stmt.GetInt64("verified") != 0,
				TokenKey: stmt.GetText("tokenKey"),
			}
			return nil
		},
		user.Email,    // 1. email
		user.Password, // 2. password
		user.Name,     // 3. name
		user.TokenKey) // 4. tokenKey

	if err != nil {
		// Check for SQLITE_CONSTRAINT_UNIQUE (2067) error code
		if sqliteErr, ok := err.(sqlite.Error); ok {
			if sqliteErr.Code == sqlite.SQLITE_CONSTRAINT_UNIQUE {
				return nil, db.ErrConstraintUnique
			}
		}
		return nil, err
	}

	return createdUser, err
}
