package moderncsqlite

import (
	"testing"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/dbtest"
)

func TestLogSuite(t *testing.T) {
	dbtest.LogSuite{New: func(t *testing.T) db.DbLog {
		logDB, _ := newTestLogDB(t)
		return logDB
	}}.RunAll(t)
}
