package crawshaw

import (
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
	"fmt"
	"runtime"

	"github.com/caasmo/restinpieces/db"
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

// Close implements db.Db interface and releases all database resources.
// It should be called when the database connection is no longer needed.
func (d *Db) Close() {
	d.pool.Close()
}

func (d *Db) UpdatePassword(userId string, newPassword string) error {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	return sqlitex.Execute(conn,
		`UPDATE users 
		SET password = ?,
			updated = (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
		WHERE id = ?`,
		nil,
		newPassword,
		userId)
}
