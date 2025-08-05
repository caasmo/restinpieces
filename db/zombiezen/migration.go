package zombiezen

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// ApplyMigrations executes all .sql files from the given filesystem against the database connection.
// It walks the directory structure recursively.
func ApplyMigrations(conn *sqlite.Conn, fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // Propagate errors from WalkDir
		}
		if d.IsDir() || filepath.Ext(path) != ".sql" {
			return nil // Skip directories and non-sql files
		}

		sqlBytes, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("could not read embedded migration file %s: %w", path, err)
		}

		if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
			return fmt.Errorf("failed to execute migration file %s: %w", path, err)
		}
		return nil
	})
}
