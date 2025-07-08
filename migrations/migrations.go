package migrations

import (
	"embed"
	"io/fs"
)

//go:embed schema/**/*.sql
var schemaFS embed.FS

// Schema returns the embedded schema filesystem
func Schema() fs.FS {
	fs, err := fs.Sub(schemaFS, "schema")
	if err != nil {
		panic(err) // should never happen since we control the embed path
	}
	return fs
}
