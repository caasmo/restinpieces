package sql

import (
	"embed"
	"io/fs"
)

//go:embed schema/**/*.sql
var sqlFS embed.FS

// FS returns the embedded filesystem containing the SQL files.
func FS() fs.FS {
	fs, err := fs.Sub(sqlFS, "schema")
	if err != nil {
		panic(err)
	}
	return fs
}
