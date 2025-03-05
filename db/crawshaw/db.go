package crawshaw

import (
	"github.com/caasmo/restinpieces/db"
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

