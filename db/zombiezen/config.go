package zombiezen

import (
	"context"
	"fmt"
	"io"
	"time" // Add time import

	"github.com/caasmo/restinpieces/db" // Import db for TimeFormat
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// LatestConfig retrieves the latest encrypted configuration blob for the specified scope.
// Returns nil slice if no config exists for the given scope (no error).
func (d *Db) LatestConfig(scope string) ([]byte, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to get db connection for scope '%s': %w", scope, err)
	}
	defer d.pool.Put(conn)

	var encryptedData []byte
	err = sqlitex.Execute(conn,
		`SELECT content FROM app_config
		 WHERE scope = ?
		 ORDER BY created_at DESC
		 LIMIT 1;`,
		&sqlitex.ExecOptions{
			Args: []any{scope}, // Bind the scope parameter
			ResultFunc: func(stmt *sqlite.Stmt) (err error) {
				// Get a reader for the blob column (index 0) - content
				reader := stmt.ColumnReader(0)
				// Read all data from the reader
				encryptedData, err = io.ReadAll(reader)
				return err // Return any error from io.ReadAll
			},
		})

	if err != nil {
		return nil, fmt.Errorf("failed to get latest config for scope '%s': %w", scope, err)
	}

	// encryptedData will be nil if no row was found, which is the desired behavior
	return encryptedData, nil
}

// InsertConfig inserts a new encrypted configuration blob into the database.
func (d *Db) InsertConfig(scope string, encryptedData []byte, format string, description string) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get db connection for insert: %w", err)
	}
	defer d.pool.Put(conn)

	now := db.TimeFormat(time.Now()) // Use db.TimeFormat for consistency

	err = sqlitex.Execute(conn,
		`INSERT INTO app_config (
			scope,
			content,
			format,
			description,
			created_at
		) VALUES (?, ?, ?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []interface{}{
				scope,
				encryptedData,
				format,
				description,
				now,
			},
		})

	if err != nil {
		// Check for unique constraint violation if needed, otherwise return generic error
		return fmt.Errorf("failed to insert config for scope '%s': %w", scope, err)
	}

	return nil
}
