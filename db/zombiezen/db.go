package zombiezen

import (
	"context"
	"fmt"
	"github.com/caasmo/restinpieces/db"
	"runtime"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type Db struct {
	pool *sqlitex.Pool
	//rwConn *sqlitex.Conn
	rwCh chan *sqlite.Conn
}

// Verify interface implementation (non-allocating check)
var _ db.Db = (*Db)(nil)

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

	conn, err := p.Take(context.TODO())
	if err != nil {
		return nil, err
	}
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
	conn, err := db.pool.Take(context.TODO())
	if err != nil {
		panic(err) // TODO: Proper error handling
	}
	defer db.pool.Put(conn)

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

func (db *Db) Insert(value int64) {
	rwConn := <-db.rwCh
	defer func() { db.rwCh <- rwConn }()

	if err := sqlitex.Execute(rwConn, "INSERT INTO foo(id, value) values(1000000,?)", &sqlitex.ExecOptions{
		Args: []any{value},
	}); err != nil {
		// TODO
		panic(err)
	}
}

func (db *Db) InsertWithPool(value int64) {
	conn, err := db.pool.Take(context.TODO())
	if err != nil {
		panic(err) // TODO: Proper error handling
	}
	defer db.pool.Put(conn)

	if err := sqlitex.Execute(conn, "INSERT INTO foo(id, value) values(1000000,?)", &sqlitex.ExecOptions{
		Args: []any{value},
	}); err != nil {
		// TODO
		panic(err)
	}
}
