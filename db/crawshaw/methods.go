package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"github.com/caasmo/restinpieces/db"
	"time"
)

// Verify interface implementation (non-allocating check)
var _ db.Db = (*Db)(nil)

func (db *Db) Close() {
	db.pool.Close()
}

func (db *Db) GetUserByEmail(email string) (string, string, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var userID, hashedPassword string
	err := sqlitex.Exec(conn, 
		"SELECT id, password FROM users WHERE email = ? LIMIT 1",
		func(stmt *sqlite.Stmt) error {
			userID = stmt.GetText("id")
			hashedPassword = stmt.GetText("password")
			return nil
		}, email)

	return userID, hashedPassword, err
}

func (db *Db) GetById(id int64) int {
	conn := db.pool.Get(nil)
	defer db.pool.Put(conn)

	var value int
	fn := func(stmt *sqlite.Stmt) error {
		value = int(stmt.GetInt64("value"))
		return nil
	}

	if err := sqlitex.Exec(conn, "select value from foo where rowid = ? limit 1", fn, any(id)); err != nil {
		panic(err)
	}
	return value
}

func (db *Db) Insert(value int64) {
	rwConn := <-db.rwCh
	defer func() { db.rwCh <- rwConn }()

	if err := sqlitex.Exec(rwConn, "INSERT INTO foo(id, value) values(1000000,?)", nil, any(value)); err != nil {
		panic(err)
	}
}

func (db *Db) InsertWithPool(value int64) {
	conn := db.pool.Get(nil)
	defer db.pool.Put(conn)

	if err := sqlitex.Exec(conn, "INSERT INTO foo(id, value) values(1000000,?)", nil, any(value)); err != nil {
		panic(err)
	}
}

// CreateUser inserts a new user with RFC3339 formatted UTC timestamps.
// The Created and Updated fields will be set automatically using time.Now().UTC().Format(time.RFC3339)
// Example timestamp: "2024-03-07T15:04:05Z"
func (d *Db) CreateUser(email, hashedPassword, name string) (*db.User, error) {
	conn := db.pool.Get(nil)
	defer db.pool.Put(conn)
	
	var user db.User
	// Generate timestamps before insert
	now := time.Now().UTC().Format(time.RFC3339)
	
	err := sqlitex.Exec(conn, 
		`INSERT INTO users (email, password, name, created, updated) 
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, email, name, password, created, updated, verified, token_key`,
		func(stmt *sqlite.Stmt) error {
			user = db.User{
				ID:        stmt.GetText("id"),
				Email:     stmt.GetText("email"),
				Name:      stmt.GetText("name"),
				Password:  stmt.GetText("password"),
				Created:   stmt.GetText("created"),   // Will match our inserted RFC3339 value
				Updated:   stmt.GetText("updated"),   // Will match our inserted RFC3339 value
				Verified:  stmt.GetBool("verified"),
				TokenKey:  stmt.GetText("token_key"),
			}
			return nil
		}, email, hashedPassword, name, now, now)

	return &user, err
}
