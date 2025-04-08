package crawshaw

import (
	"context"
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

// Get retrieves the latest TOML serialized configuration from the database
func (d *Db) Get() (string, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var configToml string
	err := sqlitex.Execute(conn,
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

	if configToml == "" {
		return "", sqlitex.ErrNoRows
	}

	return configToml, nil
}
