package moderncsqlite

import (
	"context"
	"database/sql/driver"
	"fmt"

	"modernc.org/sqlite"
)

// NewConn opens a single SQLite connection straight through the modernc
// driver, bypassing the database/sql stdlib layer: no pool, no Raw, no
// stdlib argument re-wrapping. The dsn is the full modernc DSN, including
// pragmas, e.g.:
//
//	"file:app.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&mode=rw"
//
// The driver applies the _pragma parameters on every new connection itself.
// The database file must already exist: mode=rw in the DSN makes SQLite fail
// instead of creating the file.
func NewConn(dsn string) (driver.Conn, error) {
	connector, err := sqlite.NewConnector(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create connector: %w", err)
	}

	conn, err := connector.Connect(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return conn, nil
}
