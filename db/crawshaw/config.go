package crawshaw

import (
	"context"
	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

// Get retrieves the TOML serialized configuration from the database
func (d *Db) Get() (string, error) {
	conn, err := d.pool.Take(context.TODO())
	if err != nil {
		return "", err
	}
	defer d.pool.Put(conn)

	var configToml string
	err = sqlitex.Execute(conn,
		`SELECT value FROM config WHERE key = 'app';`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				configToml = stmt.GetText("value")
				return nil
			},
		})

	if err != nil {
		return "", err
	}

	if configToml == "" {
		return "", sqlitex.ErrNoRows
	}

	return configToml, nil
}
