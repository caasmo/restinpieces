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

// Verify interface implementation (non-allocating check)
var _ db.Db = (*Db)(nil)

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

