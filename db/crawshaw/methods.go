package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"encoding/json"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"strings"
	"time"
)


// GetUserByEmail retrieves a user by email address.
// Returns:
// - *db.User: User record if found, nil if no matching record exists
// - error: Only returned for database errors, nil on successful query (even if no results)
// Note: A nil user with nil error indicates no matching record was found
func (d *Db) GetUserByEmail(email string) (*db.User, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var user *db.User  // Will remain nil if no rows found
	err := sqlitex.Exec(conn, 
		`SELECT id, email, name, password, created, updated, verified, tokenKey 
		FROM users WHERE email = ? LIMIT 1`,
		func(stmt *sqlite.Stmt) error {
			user = &db.User{
				ID:        stmt.GetText("id"),
				Email:     stmt.GetText("email"),
				Name:      stmt.GetText("name"),
				Password:  stmt.GetText("password"),
				Created:   stmt.GetText("created"),
				Updated:   stmt.GetText("updated"),
				Verified:  stmt.GetInt64("verified") != 0,
				TokenKey:  stmt.GetText("tokenKey"),
			}
			return nil
		}, email)

	if err != nil {
		return nil, err
	}

	return user, nil
}


// validateUserFields checks that required user fields are present
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
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	payloadJSON, err := json.Marshal(job.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)

	err = sqlitex.Exec(conn, `INSERT OR IGNORE INTO job_queue 
		(job_type, payload, status, attempts, max_attempts, 
		created_at, updated_at, scheduled_for) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		nil, // No results needed for INSERT
		job.JobType,
		string(payloadJSON),
		queue.StatusPending,
		job.Attempts,
		job.MaxAttempts,
		now,
		now,
		job.ScheduledFor.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("queue insert failed: %w", err)
	}
	return nil
}

// CreateUser inserts a new user with RFC3339 formatted UTC timestamps.
// The Created and Updated fields will be set automatically using time.Now().UTC().Format(time.RFC3339)
// Example timestamp: "2024-03-07T15:04:05Z"
// User struct should contain at minimum: Email, Password (pre-hashed), and Name
func (d *Db) CreateUser(user db.User) (*db.User, error) {
	// Validate required fields
	if err := validateUserFields(user); err != nil {
		return nil, err
	}

	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)
	
	// Generate timestamps before insert
	//TODO utc shoudl be already in user 
	now := time.Now().UTC().Format(time.RFC3339)
	
	var createdUser *db.User
	err := sqlitex.Exec(conn, 
		`INSERT INTO users (email, password, name, created, updated, tokenKey) 
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id, email, name, password, created, updated, verified, tokenKey`,
		func(stmt *sqlite.Stmt) error {
			createdUser = &db.User{
				ID:        stmt.GetText("id"),
				Email:     stmt.GetText("email"),
				Name:      stmt.GetText("name"),
				Password:  stmt.GetText("password"),
				Created:   stmt.GetText("created"),
				Updated:   stmt.GetText("updated"),
				Verified:  stmt.GetInt64("verified") != 0,
				TokenKey:  stmt.GetText("tokenKey"),
			}
			return nil
		},
		user.Email,   // 1. email
		user.Password, // 2. password
		user.Name,    // 3. name
		now,          // 4. created
		now,          // 5. updated
		user.TokenKey) // 6. tokenKey

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
