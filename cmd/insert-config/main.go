package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type ConfigInserter struct {
	dbfile string
	logger *slog.Logger
	pool   *sqlitex.Pool
}

func NewConfigInserter(dbfile string) *ConfigInserter {
	return &ConfigInserter{
		dbfile: dbfile,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

func (ci *ConfigInserter) OpenDatabase() error {
	pool, err := sqlitex.NewPool(ci.dbfile, sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadWrite,
		PoolSize: runtime.NumCPU(),
	})
	if err != nil {
		ci.logger.Error("failed to open database", "error", err)
		return err
	}
	ci.pool = pool
	return nil
}

func (ci *ConfigInserter) InsertConfig(tomlPath string) error {
	// Read config file
	configData, err := os.ReadFile(tomlPath)
	if err != nil {
		ci.logger.Error("failed to read config file", "path", tomlPath, "error", err)
		return err
	}

	conn, err := ci.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer ci.pool.Put(conn)

	now := time.Now().UTC().Format(time.RFC3339)
	description := "Inserted from file: " + filepath.Base(tomlPath)

	err = sqlitex.Execute(conn,
		`INSERT INTO app_config (
			content, 
			format,
			description,
			created_at
		) VALUES (?, ?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []interface{}{
				string(configData),  // content
				"toml",             // format
				description,        // description
				now,                // created_at
			},
		})

	if err != nil {
		ci.logger.Error("failed to insert config", "error", err)
		return err
	}

	ci.logger.Info("successfully inserted config")
	return nil
}

func main() {
	if len(os.Args) != 3 {
		slog.Error("usage: insert-config <toml-file> <db-file>")
		os.Exit(1)
	}

	tomlPath := os.Args[1]
	dbPath := os.Args[2]

	inserter := NewConfigInserter(dbPath)
	if err := inserter.OpenDatabase(); err != nil {
		os.Exit(1)
	}
	defer inserter.pool.Close()

	if err := inserter.InsertConfig(tomlPath); err != nil {
		os.Exit(1)
	}
}
