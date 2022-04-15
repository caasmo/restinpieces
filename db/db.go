package db

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

//
func New(path string) (*Db, error) {
	poolSize := runtime.NumCPU()
	initString := fmt.Sprintf("file:%s", path)

	p, err := sqlitex.Open(initString, 0, poolSize)
	if err != nil {
		return &Db{}, err
	}

	conn := p.Get(nil)
	// TODO keep track of closing
	//defer db.Put(conn)
	ch := make(chan *sqlite.Conn, 1)
	go func(conn *sqlite.Conn, ch chan *sqlite.Conn) {
		// TODO Use context to select with timeout to cleanly clean up goroutine
		ch <- conn
	}(conn, ch)

	return &Db{pool: p, rwCh: ch}, nil
}

func (db *Db) Close() {
	db.Close()
}

func (db *Db) GetById(id int64) int {
	conn := db.pool.Get(nil)
	defer db.pool.Put(conn)

	var value int
	fn := func(stmt *sqlite.Stmt) error {
		//id = int(stmt.GetInt64("id"))
		value = int(stmt.GetInt64("value"))
		return nil
	}

	if err := sqlitex.Exec(conn, "select value from foo where rowid = ? limit 1", fn, id); err != nil {
		// TODO
		panic(err)
	}

	return value
}

func (db *Db) Insert(value int64) {
	rwConn := <-db.rwCh
	defer func() { db.rwCh <- rwConn }()

	if err := sqlitex.Exec(rwConn, "INSERT INTO foo(id, value) values(1000000,?)", nil, value); err != nil {
		// TODO
		panic(err)
	}
}

func (db *Db) InsertWithPool(value int64) {
	conn := db.pool.Get(nil)
	defer db.pool.Put(conn)

	if err := sqlitex.Exec(conn, "INSERT INTO foo(id, value) values(1000000,?)", nil, value); err != nil {
		// TODO
		panic(err)
	}
}
