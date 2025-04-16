package zombiezen

// TODO not code reviewed. Machine generated R1

import (
	"context"
	"fmt"
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
			ResultFunc: func(stmt *sqlite.Stmt) error {
				// Get the length of the blob column (index 0)
				length := stmt.ColumnLen(0)
				// Allocate a buffer with the exact size needed
				encryptedData = make([]byte, length)
				// Read the blob content directly into the buffer using ColumnBytes
				n := stmt.ColumnBytes(0, encryptedData)
				// Check if the number of bytes read matches the expected length
				if n != length {
					return fmt.Errorf("ColumnBytes read %d bytes, expected %d", n, length)
				}
				return nil // Success
			},
		})

	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	// encryptedData will be nil if no row was found, which is the desired behavior
	return encryptedData, nil
}
