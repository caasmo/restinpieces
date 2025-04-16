package zombiezen

// TODO not code reviewed. Machine generated R1

import (
	"context"
	"fmt"
	"io" // Add io import
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// GetConfig retrieves the latest encrypted configuration blob from the database.
// Returns nil slice if no config exists (no error).
func (d *Db) GetConfig() ([]byte, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to get db connection: %w", err)
	}
	defer d.pool.Put(conn)

	var encryptedData []byte
	err = sqlitex.Execute(conn,
		`SELECT content FROM app_config
		ORDER BY created_at DESC
		LIMIT 1;`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) (err error) {
				// Get a reader for the blob column (index 0)
				reader := stmt.ColumnReader(0)
				// Read all data from the reader
				encryptedData, err = io.ReadAll(reader)
				return err // Return any error from io.ReadAll
			},
		})

	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	// encryptedData will be nil if no row was found, which is the desired behavior
	return encryptedData, nil
}
