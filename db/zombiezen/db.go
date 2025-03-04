package zombiezen

import (
	"context"
	"github.com/zombiezen/go-sqlite"
	"github.com/zombiezen/go-sqlite/sqlitex"
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

	p, err := sqlitex.NewPool(initString, sqlitex.PoolOptions{
		Flags:    0, // Use all default flags including WAL
		PoolSize: poolSize,
	})
	if err != nil {
		return nil, err
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
	db.pool.Close()
}

func (db *Db) GetById(id int64) int {
	conn := db.pool.Get(nil)
	defer db.pool.Put(conn)

	var value int
	fn := func(stmt *sqlite.Stmt) error {
		//id = int(stmt.GetInt64("id"))
		value = int(stmt.ColumnInt64(0))
		return nil
	}

	if err := sqlitex.Execute(conn, "select value from foo where rowid = ? limit 1", &sqlitex.ExecOptions{
		ResultFunc: fn,
		Args:       []interface{}{id},
	}); err != nil {
		// TODO
		panic(err)
	}

	return value
}

func (db *Db) Insert(value int64) {
	rwConn := <-db.rwCh
	defer func() { db.rwCh <- rwConn }()

	if err := sqlitex.Execute(rwConn, "INSERT INTO foo(id, value) values(1000000,?)", &sqlitex.ExecOptions{
		Context: context.Background(),
		Args:    []interface{}{value},
	}); err != nil {
		// TODO
		panic(err)
	}
}

func (db *Db) InsertWithPool(value int64) {
	conn := db.pool.Get(nil)
	defer db.pool.Put(conn)

	if err := sqlitex.Execute(conn, "INSERT INTO foo(id, value) values(1000000,?)", &sqlitex.ExecOptions{
		Context: context.Background(),
		Args:    []interface{}{value},
	}); err != nil {
		// TODO
		panic(err)
	}
}
