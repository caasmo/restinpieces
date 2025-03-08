package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"github.com/caasmo/restinpieces/db"
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


// CreateUser inserts a new user with RFC3339 formatted UTC timestamps.
// The Created and Updated fields will be set automatically using time.Now().UTC().Format(time.RFC3339)
// Example timestamp: "2024-03-07T15:04:05Z"
// User struct should contain at minimum: Email, Password (pre-hashed), and Name
func (d *Db) CreateUser(user db.User) (*db.User, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)
	
	// Generate timestamps before insert
	now := time.Now().UTC().Format(time.RFC3339)
	
	fmt.Printf("Attempting to create user with params:\nemail: %q\npassword: %q\nname: %q\ncreated: %q\nupdated: %q\n",
		user.Email, user.Password, user.Name, now, now)
	
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
			fmt.Printf("Created user: %+v\n", user)
			return nil
		},
		user.Email,   // 1. email
		user.Password, // 2. password
		user.Name,    // 3. name
		now,          // 4. created
		now,          // 5. updated
		user.TokenKey) // 6. tokenKey

	if err != nil {
		fmt.Printf("SQL error: %v\n", err)
	}
	
	return createdUser, err
}
