package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"runtime"
)

type Db struct {
	pool *sqlitex.Pool
	//rwConn *sqlitex.Conn
	rwCh chan *sqlite.Conn
}

func New(path string) (*Db, error) {
	poolSize := runtime.NumCPU()
	initString := fmt.Sprintf("file:%s", path)

	p, err := sqlitex.Open(initString, 0, poolSize)
	if err != nil {
		return &Db{}, err
	}

	conn := p.Get(nil)

	ch := make(chan *sqlite.Conn, 1)
	go func(conn *sqlite.Conn, ch chan *sqlite.Conn) {
		ch <- conn
	}(conn, ch)

	return &Db{pool: p, rwCh: ch}, nil
}

func (db *Db) GetUserByEmail(email string) (string, string, error) {
	conn := db.pool.Get(nil)
	defer db.pool.Put(conn)

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
