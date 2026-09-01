package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/caasmo/restinpieces"
	dbm "github.com/caasmo/restinpieces/db/databasesql"
	sqlfs "github.com/caasmo/restinpieces/sql"
)

var (
	ErrCreateDbPool = errors.New("failed to create database pool")
	ErrCreateDbImpl = errors.New("failed to instantiate modernc db from pool")
	ErrApplyLogSQL  = errors.New("failed to apply log SQL")
)

// appDb is the app database for ripc. It embeds the modernc app DB
// and adds ad-hoc query helpers used only by the ripc CLI.
type appDb struct {
	db *sql.DB
	*dbm.Db
}

func newAppDb(dbPath string) (*appDb, error) {
	db, err := restinpieces.NewModerncPool(dbPath)
	if err != nil {
		return nil, fmt.Errorf("%w (db_path: %s): %v", ErrCreateDbPool, dbPath, err)
	}
	mdb, err := dbm.New(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%w: %v", ErrCreateDbImpl, err)
	}
	return &appDb{db: db, Db: mdb}, nil
}

func (db *appDb) Close() error {
	return db.db.Close()
}

type configRow struct {
	Scope       string
	CreatedAt   string
	Format      string
	Description string
}

func (db *appDb) configList(scopeFilter string) (rows []configRow, err error) {
	query := "SELECT scope, created_at, format, description FROM app_config ORDER BY created_at DESC;"
	if scopeFilter != "" {
		query = "SELECT scope, created_at, format, description FROM app_config WHERE scope = ? ORDER BY created_at DESC;"
	}

	var qrows *sql.Rows
	if scopeFilter != "" {
		qrows, err = db.db.Query(query, scopeFilter)
	} else {
		qrows, err = db.db.Query(query)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: failed to query config for list command", ErrQueryPrepare)
	}
	defer func() {
		if ferr := qrows.Close(); ferr != nil {
			err = errors.Join(err, fmt.Errorf("%w: failed to close list results: %w", ErrDbFinalize, ferr))
		}
	}()

	for qrows.Next() {
		var row configRow
		if err := qrows.Scan(&row.Scope, &row.CreatedAt, &row.Format, &row.Description); err != nil {
			return nil, fmt.Errorf("%w: failed to scan list results: %w", ErrDbStep, err)
		}
		rows = append(rows, row)
	}
	if err := qrows.Err(); err != nil {
		return nil, fmt.Errorf("%w: failed to iterate list results: %w", ErrDbStep, err)
	}

	return rows, nil
}

func (db *appDb) configScopes() (scopes []string, err error) {
	qrows, err := db.db.Query("SELECT DISTINCT scope FROM app_config ORDER BY scope;")
	if err != nil {
		return nil, fmt.Errorf("%w: for scopes command: %w", ErrDbPrepare, err)
	}
	defer func() {
		if ferr := qrows.Close(); ferr != nil {
			err = errors.Join(err, fmt.Errorf("%w: %w", ErrDbFinalize, ferr))
		}
	}()

	for qrows.Next() {
		var scope string
		if err := qrows.Scan(&scope); err != nil {
			return nil, fmt.Errorf("%w: for scopes command: %w", ErrDbStep, err)
		}
		scopes = append(scopes, scope)
	}
	if err := qrows.Err(); err != nil {
		return nil, fmt.Errorf("%w: for scopes command: %w", ErrDbStep, err)
	}

	return scopes, nil
}

// TODO: refactor — createSchemas should not be a method on appDb; move to standalone helper or driver package (keep sql.go pure)
func (db *appDb) createSchemas() error {
	if err := applySQL(db.db, "app"); err != nil {
		return fmt.Errorf("%w: sql process failed: %w", ErrApplySQL, err)
	}
	return nil
}

// logDb is the log database for ripc. It mirrors appDb so both databases
// share the same constructor + createSchemas + Close shape.
type logDb struct {
	db *sql.DB
}

func newLogDb(dbPath string) (*logDb, error) {
	db, err := restinpieces.NewModerncPool(dbPath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to open/create log database at %s: %w", ErrDbConnection, dbPath, err)
	}
	return &logDb{db: db}, nil
}

func (db *logDb) Close() error {
	return db.db.Close()
}

func (db *logDb) createSchemas() error {
	if err := applySQL(db.db, "log"); err != nil {
		return fmt.Errorf("%w: failed to execute log sql: %w", ErrApplyLogSQL, err)
	}
	return nil
}

// applySQL executes all .sql files in the given directory of the embedded
// SQL filesystem. The embedded filesystem is expected to contain only one
// level of directories (app, log). Each file may contain multiple
// statements; the modernc driver executes them in a single Exec call.
func applySQL(db *sql.DB, dir string) error {
	fsys, err := fs.Sub(sqlfs.FS(), dir)
	if err != nil {
		return fmt.Errorf("could not access embedded sql dir %s: %w", dir, err)
	}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("could not read embedded sql dir %s: %w", dir, err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".sql" {
			continue
		}

		sqlBytes, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return fmt.Errorf("could not read embedded sql file %s/%s: %w", dir, e.Name(), err)
		}

		_, execErr := db.Exec(string(sqlBytes))
		if execErr != nil {
			return fmt.Errorf("failed to execute sql file %s/%s: %w", dir, e.Name(), execErr)
		}
	}
	return nil
}
