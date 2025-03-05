package crawshaw

import (
	"github.com/caasmo/restinpieces/db"
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

// Verify interface implementation (non-allocating check)
var _ db.Db = (*Db)(nil)

func (db *Db) Close() {
	db.pool.Close()
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
