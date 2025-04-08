package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

// TODO deprecated

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

func (d *Db) InsertWithPool(value int64) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	if err := sqlitex.Exec(conn, "INSERT INTO foo(id, value) values(1000000,?)", nil, any(value)); err != nil {
		panic(err)
	}
}
