package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/caasmo/restinpieces"
	dbz "github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/sql"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

var (
	ErrCreateDbPool = errors.New("failed to create database pool")
	ErrCreateDbImpl = errors.New("failed to instantiate zombiezen db from pool")
	ErrApplyLogSQL  = errors.New("failed to apply log SQL")
)

// appDb is the app database for ripc. It embeds the zombiezen app DB
// and adds ad-hoc query helpers used only by the ripc CLI.
type appDb struct {
	pool *sqlitex.Pool
	*dbz.Db
}

func newAppDb(dbPath string) (*appDb, error) {
	pool, err := restinpieces.NewZombiezenPool(dbPath)
	if err != nil {
		return nil, fmt.Errorf("%w (db_path: %s): %v", ErrCreateDbPool, dbPath, err)
	}
	zdb, err := dbz.New(pool)
	if err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("%w: %v", ErrCreateDbImpl, err)
	}
	return &appDb{pool: pool, Db: zdb}, nil
}

// TODO: refactor this — test-only helper, move to test helper or remove
func newAppDbFromPool(pool *sqlitex.Pool) *appDb {
	zdb, _ := dbz.New(pool)
	return &appDb{pool: pool, Db: zdb}
}

func (db *appDb) Close() error {
	return db.pool.Close()
}

type configRow struct {
	Scope       string
	CreatedAt   string
	Format      string
	Description string
}

func (db *appDb) configList(scopeFilter string) (rows []configRow, err error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get db connection for list command", ErrDbConnection)
	}
	defer db.pool.Put(conn)

	query := "SELECT scope, created_at, format, description FROM app_config ORDER BY created_at DESC;"
	if scopeFilter != "" {
		query = "SELECT scope, created_at, format, description FROM app_config WHERE scope = ? ORDER BY created_at DESC;"
	}

	stmt, err := conn.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to prepare statement for list command", ErrQueryPrepare)
	}
	defer func() {
		if ferr := stmt.Finalize(); ferr != nil {
			err = errors.Join(err, fmt.Errorf("failed to finalize statement: %w", ferr))
		}
	}()

	if scopeFilter != "" {
		stmt.BindText(1, scopeFilter)
	}

	for {
		hasRow, stepErr := stmt.Step()
		if stepErr != nil {
			return nil, fmt.Errorf("failed to step through list results: %w", stepErr)
		}
		if !hasRow {
			break
		}

		rows = append(rows, configRow{
			Scope:       stmt.GetText("scope"),
			CreatedAt:   stmt.GetText("created_at"),
			Format:      stmt.GetText("format"),
			Description: stmt.GetText("description"),
		})
	}

	return rows, err
}

func (db *appDb) configScopes() (scopes []string, err error) {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return nil, fmt.Errorf("%w: for scopes command: %w", ErrDbConnection, err)
	}
	defer db.pool.Put(conn)

	stmt, err := conn.Prepare("SELECT DISTINCT scope FROM app_config ORDER BY scope;")
	if err != nil {
		return nil, fmt.Errorf("%w: for scopes command: %w", ErrDbPrepare, err)
	}
	defer func() {
		if ferr := stmt.Finalize(); ferr != nil {
			err = errors.Join(err, fmt.Errorf("%w: %w", ErrDbFinalize, ferr))
		}
	}()

	for {
		hasRow, stepErr := stmt.Step()
		if stepErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrDbStep, stepErr)
		}
		if !hasRow {
			break
		}
		scopes = append(scopes, stmt.GetText("scope"))
	}

	return scopes, err
}

// TODO: refactor — applyAppSchema should not be a method on appDb; move to standalone helper or driver package (keep sql.go pure)
func (db *appDb) applyAppSchema() error {
	conn, err := db.pool.Take(context.Background())
	if err != nil {
		return fmt.Errorf("%w: for sql: %w", ErrDbConnection, err)
	}
	defer db.pool.Put(conn)

	if err := applySQL(conn, "app"); err != nil {
		return fmt.Errorf("%w: sql process failed: %w", ErrApplySQL, err)
	}

	return nil
}

func initLogDb(logDbPath string) error {
	pool, err := restinpieces.NewZombiezenPool(logDbPath)
	if err != nil {
		return fmt.Errorf("%w: failed to open/create log database at %s: %w", ErrDbConnection, logDbPath, err)
	}
	defer func() {
		_ = pool.Close()
	}()

	conn, err := pool.Take(context.Background())
	if err != nil {
		return fmt.Errorf("%w: failed to get connection from pool: %w", ErrDbConnection, err)
	}
	defer pool.Put(conn)

	err = applySQL(conn, "log")
	if err != nil {
		return fmt.Errorf("%w: failed to execute log sql: %w", ErrApplyLogSQL, err)
	}

	return nil
}

// applySQL executes all .sql files in the given directory of the embedded SQL filesystem.
// The embedded filesystem is expected to contain only one level of directories (app, log).
func applySQL(conn *sqlite.Conn, dir string) error {
	fsys, err := fs.Sub(sql.FS(), dir)
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

		if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
			return fmt.Errorf("failed to execute sql file %s/%s: %w", dir, e.Name(), err)
		}
	}
	return nil
}
