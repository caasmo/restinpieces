package crawshaw

import (
	"fmt"
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

// GetConfig retrieves the latest TOML serialized configuration from the database.
// Returns empty string if no config exists (no error).
func (d *Db) GetConfig() (string, error) {
	conn := d.pool.Get(nil)
	defer d.pool.Put(conn)

	var configToml string
	err := sqlitex.Exec(conn,
		`SELECT content FROM app_config 
		ORDER BY created_at DESC 
		LIMIT 1;`,
		nil,
		func(stmt *sqlite.Stmt) error {
			configToml = stmt.GetText("content")
			return nil
		})

	if err != nil {
		return "", fmt.Errorf("config: failed to get: %w", err)
	}

	return configToml, nil
}
