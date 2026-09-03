package databasesql

import (
	"database/sql"
	"fmt"

	"github.com/caasmo/restinpieces/db"
)

type Db struct {
	db    *sql.DB
	stmts map[string]*sql.Stmt // statement name -> prepared statement
}

// Verify interface implementations
var (
	_ db.DbAuth   = (*Db)(nil)
	_ db.DbQueue  = (*Db)(nil)
	_ db.DbConfig = (*Db)(nil)
)

// New creates a new Db instance using an existing database provided by the
// user. It does not prepare statements; call RegisterStmt during startup,
// before the Db runs statements. Note: The lifecycle of the provided *sql.DB
// is managed externally. This Db type does not close the database.
func New(sqlDB *sql.DB) (*Db, error) {
	if sqlDB == nil {
		return nil, fmt.Errorf("provided database cannot be nil")
	}
	return &Db{db: sqlDB, stmts: make(map[string]*sql.Stmt)}, nil
}

// RegisterStmt prepares the statement for sql, stores it under name, and
// returns it. Call it once at startup, before the Db runs statements.
// Calling it again with the same name returns the stored statement.
//
// The statement is prepared with db.Prepare, so it belongs to the pool, not
// to any single connection: every execution runs on whichever pool
// connection is free. database/sql caches such statements internally per
// connection — its css list, one prepared driver statement per connection —
// so the SQL is parsed once per connection and reused by every later call
// that lands on that connection.
//
// Because executions land on whichever connection is free, a registered
// statement can never run inside a transaction: a transaction lives on a
// single pinned connection, and the pool statement would execute outside it
// (or block, if no connection is free). Register here only statements that
// run standalone (autocommit). A statement that must run inside a
// transaction is prepared on the pinned connection with conn.PrepareContext
// instead — it is bound to that one connection, has no cache, and is
// prepared again on every call.
func (d *Db) RegisterStmt(name, sql string) (*sql.Stmt, error) {
	if s, ok := d.stmts[name]; ok {
		return s, nil
	}

	s, err := d.db.Prepare(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	d.stmts[name] = s
	return s, nil
}

// Stmt returns the statement registered for name.
// Db methods and app developers run their statements through the statement
// stored here.
func (d *Db) Stmt(name string) (*sql.Stmt, error) {
	s, ok := d.stmts[name]
	if !ok {
		return nil, fmt.Errorf("no statement registered for %q: call RegisterStmt at startup", name)
	}
	return s, nil
}
