package main

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/caasmo/restinpieces/sql"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

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
