package zombiezen

// TODO not code reviewed. Machine generated R1

import (
	"context"
	"fmt"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// GetConfig retrieves the latest TOML serialized configuration from the database.
// Returns empty string if no config exists (no error).
func (d *Db) GetConfig() (string, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to get db connection: %w", err)
	}
	defer d.pool.Put(conn)

	var configToml string
	err = sqlitex.Execute(conn,
		`SELECT content FROM app_config 
		ORDER BY created_at DESC 
		LIMIT 1;`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				configToml = stmt.GetText("content")
				return nil
			},
		})

	if err != nil {
		return "", fmt.Errorf("failed to get config: %w", err)
	}

	return configToml, nil
}
