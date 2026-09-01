package databasesql

import (
	"database/sql"
	"errors"
	"fmt"
)

// GetConfig retrieves the latest configuration content blob for the specified scope.
// Returns nil slice if no config exists for the given scope (no error).
func (d *Db) GetConfig(scope string, generation int) ([]byte, string, error) {
	var (
		content []byte
		format  string
	)
	err := d.db.QueryRow(
		`SELECT content, format FROM app_config 
		 WHERE scope = ? 
		 ORDER BY created_at DESC, id DESC
		 LIMIT 1 OFFSET ?`,
		scope, generation,
	).Scan(&content, &format)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to get config for scope '%s' generation %d: %w", scope, generation, err)
	}
	return content, format, nil
}

// InsertConfig inserts a new configuration content blob into the database.
func (d *Db) InsertConfig(scope string, contentData []byte, format string, description string) error {
	_, err := d.db.Exec(
		`INSERT INTO app_config (
			scope,
			content,
			format,
			description
		) VALUES (?, ?, ?, ?)`,
		scope, contentData, format, description)
	if err != nil {
		return fmt.Errorf("failed to insert config for scope '%s': %w", scope, err)
	}
	return nil
}
