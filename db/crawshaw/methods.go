package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"time"
)

func (d *Db) GetUserByEmail(email string) (*db.User, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var user db.User
    err := sqlitex.Exec(conn, 
		`SELECT id, email, name, password, created, updated, verified, token_key 
		FROM users WHERE email = ? LIMIT 1`,
		func(stmt *sqlite.Stmt) error {
			user = db.User{
				ID:        stmt.GetText("id"),
				Email:     stmt.GetText("email"),
				Name:      stmt.GetText("name"),
				Password:  stmt.GetText("password"),
				Created:   stmt.GetText("created"),
				Updated:   stmt.GetText("updated"),
				Verified:  stmt.GetInt64("verified") != 0,
				TokenKey:  stmt.GetText("token_key"),
			}
			return nil
		}, email)

	if err != nil {
		return nil, err
	}
	return &user, nil
}


// CreateUser inserts a new user with RFC3339 formatted UTC timestamps.
// The Created and Updated fields will be set automatically using time.Now().UTC().Format(time.RFC3339)
// Example timestamp: "2024-03-07T15:04:05Z"
func (d *Db) CreateUser(email, password, name string) (*db.User, error) {
	hashedPassword, err := crypto.GenerateHash(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)
	
	var user db.User
	// Generate timestamps before insert
	now := time.Now().UTC().Format(time.RFC3339)
	
	err = sqlitex.Exec(conn, 
		`INSERT INTO users (email, password, name, created, updated) 
		VALUES (@email, @password, @name, @created, @updated)
		RETURNING id, email, name, password, created, updated, verified, token_key`,
		func(stmt *sqlite.Stmt) error {
			user = db.User{
				ID:        stmt.GetText("id"),
				Email:     stmt.GetText("email"),
				Name:      stmt.GetText("name"),
				Password:  stmt.GetText("password"),
				Created:   stmt.GetText("created"),
				Updated:   stmt.GetText("updated"),
				Verified:  stmt.GetInt64("verified") != 0,
				TokenKey:  stmt.GetText("token_key"),
			}
			return nil
		}, []interface{}{
			"@email",    email,
			"@password", hashedPassword,
			"@name",     name,
			"@created",  now,
			"@updated",  now,
		})

	return &user, err
}
