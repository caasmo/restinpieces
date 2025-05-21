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

// LatestConfig retrieves the latest configuration content blob for the specified scope.
// Returns nil slice if no config exists for the given scope (no error).
func (d *Db) GetConfig(scope string, generation int) ([]byte, string, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, "", fmt.Errorf("failed to get db connection: %w", err)
	}
	defer d.pool.Put(conn)

	var (content []byte; format string)
	err = sqlitex.Execute(conn,
		`SELECT content, format FROM app_config 
		 WHERE scope = ? 
		 ORDER BY created_at DESC 
		 LIMIT 1 OFFSET ?`,
		&sqlitex.ExecOptions{
			Args: []any{scope, generation},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				reader := stmt.ColumnReader(0)
				var err error
				content, err = io.ReadAll(reader)
				if err != nil {
					return err
				}
				format = stmt.GetText("format")
				return nil
			},
		})

	if err != nil {
		return nil, "", fmt.Errorf("failed to get config for scope '%s' generation %d: %w", scope, generation, err)
	}
	return content, format, nil
}

// InsertConfig inserts a new configuration content blob into the database.
func (d *Db) InsertConfig(scope string, contentData []byte, format string, description string) error {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to get db connection for config insert: %w", err)
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
				contentData, // Use renamed parameter
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
