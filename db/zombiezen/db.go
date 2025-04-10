package zombiezen

// TODO not code reviewed. Machine generated R1

import (
	"context"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"time"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type Db struct {
	pool *sqlitex.Pool
}

// Verify interface implementations
var _ db.DbAuth = (*Db)(nil)
var _ db.DbQueue = (*Db)(nil)
var _ db.DbConfig = (*Db)(nil)
var _ db.DbAcme = (*Db)(nil) // Added DbAcme interface check

// var _ db.DbLifecycle = (*Db)(nil) // Removed

// New creates a new Db instance using an existing pool provided by the user.
// Note: The lifecycle of the provided pool (*sqlitex.Pool) is managed externally.
// This Db type does not close the pool.
func New(pool *sqlitex.Pool) (*Db, error) {
	if pool == nil {
		return nil, fmt.Errorf("provided pool cannot be nil")
	}
	// The pool is managed externally, just store it.
	return &Db{pool: pool}, nil
}

// Close method removed as the pool lifecycle is managed externally.

func (d *Db) GetById(id int64) int { // Added missing return type 'int'
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		panic(err) // TODO: Proper error handling - return error instead of panic
	}
	defer d.pool.Put(conn)

	var value int
	fn := func(stmt *sqlite.Stmt) error {
		//id = int(stmt.GetInt64("id"))
		value = int(stmt.ColumnInt64(0))
		return nil
	}

	if err := sqlitex.Execute(conn, "select value from foo where rowid = ? limit 1", &sqlitex.ExecOptions{
		ResultFunc: fn,
		Args:       []any{id},
	}); err != nil {
		// TODO
		panic(err)
	}

	return value
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

// CreateUser inserts a new user with RFC3339 formatted UTC timestamps
// InsertJob placeholder for zombiezen SQLite implementation
func (d *Db) Claim(limit int) ([]*queue.Job, error) {
	return nil, fmt.Errorf("Claim not implemented for zombiezen SQLite variant")
}

func (d *Db) GetJobs(limit int) ([]*queue.Job, error) {
	return nil, fmt.Errorf("GetJobs not implemented for zombiezen SQLite variant")
}

func (d *Db) InsertJob(job queue.Job) error {
	return fmt.Errorf("InsertJob not implemented for zombiezen SQLite variant")
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
				true,                 // 3. verified
				true,                 // 4. oauth2
				user.Avatar,          // 5. avatar
				user.Email,           // 6. email
				user.EmailVisibility, // 7. emailVisibility
				now,                  // 8. created
				now,                  // 9. updated
			},
		})

	return &createdUser, err
}

func (d *Db) MarkCompleted(jobID int64) error {
	return fmt.Errorf("MarkCompleted not implemented for zombiezen SQLite variant")
}

func (d *Db) MarkFailed(jobID int64, errMsg string) error {
	return fmt.Errorf("MarkFailed not implemented for zombiezen SQLite variant")
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

// GetConfig retrieves the latest TOML serialized configuration from the database.
// Returns empty string if no config exists (no error).
func (d *Db) GetConfig() (string, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to get db connection: %w", err)
	}
	defer d.pool.Put(conn)

	var configToml string
	err = sqlitex.Execute(conn,
		`SELECT content FROM app_config 
		ORDER BY created_at DESC 
		LIMIT 1;`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				configToml = stmt.GetText("content")
				return nil
			},
		})

	if err != nil {
		return "", fmt.Errorf("failed to get config: %w", err)
	}

	return configToml, nil
}

func (d *Db) InsertWithPool(value int64) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		panic(err) // TODO: Proper error handling
	}
	defer d.pool.Put(conn)

	if err := sqlitex.Execute(conn, "INSERT INTO foo(id, value) values(1000000,?)", &sqlitex.ExecOptions{
		Args: []any{value},
	}); err != nil {
		// TODO
		panic(err)
	}
}
