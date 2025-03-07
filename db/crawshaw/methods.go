package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"github.com/caasmo/restinpieces/db"
	"time"
)

// Verify interface implementation (non-allocating check)
var _ db.Db = (*Db)(nil)

func (d *Db) Close() {
	d.pool.Close()
}

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

func (d *Db) GetById(id int64) int {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

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

func (d *Db) Insert(value int64) {
	rwConn := <-d.rwCh
	defer func() { d.rwCh <- rwConn }()

	if err := sqlitex.Exec(rwConn, "INSERT INTO foo(id, value) values(1000000,?)", nil, any(value)); err != nil {
		panic(err)
	}
}

func (d *Db) InsertWithPool(value int64) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	if err := sqlitex.Exec(conn, "INSERT INTO foo(id, value) values(1000000,?)", nil, any(value)); err != nil {
		panic(err)
	}
}

// CreateUser inserts a new user with RFC3339 formatted UTC timestamps.
// The Created and Updated fields will be set automatically using time.Now().UTC().Format(time.RFC3339)
// Example timestamp: "2024-03-07T15:04:05Z"
func (d *Db) CreateUser(email, hashedPassword, name string) (*db.User, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)
	
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
				Verified:  stmt.GetInt64("verified") != 0,
				TokenKey:  stmt.GetText("token_key"),
			}
			return nil
		}, email, hashedPassword, name, now, now)

	return &user, err
}
