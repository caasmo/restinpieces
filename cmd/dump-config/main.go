package main

import (
	"context"
	"log/slog"
	"os"
	"runtime"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type ConfigDumper struct {
	dbfile string
	logger *slog.Logger
	pool   *sqlitex.Pool
}

func NewConfigDumper(dbfile string) *ConfigDumper {
	return &ConfigDumper{
		dbfile: dbfile,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

func (cd *ConfigDumper) OpenDatabase() error {
	pool, err := sqlitex.NewPool(cd.dbfile, sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadWrite,
		PoolSize: runtime.NumCPU(),
	})
	if err != nil {
		cd.logger.Error("failed to open database", "error", err)
		return err
	}
	cd.pool = pool
	return nil
}

func (cd *ConfigDumper) DumpLatestConfig() (string, error) {
	conn, err := cd.pool.Take(context.Background())
	if err != nil {
		return "", err
	}
	defer cd.pool.Put(conn)

	var configContent string
	err = sqlitex.Execute(conn,
		`SELECT content FROM app_config 
		ORDER BY created_at DESC 
		LIMIT 1;`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				configContent = stmt.GetText("content")
				return nil
			},
		})

	if err != nil {
		cd.logger.Error("failed to query config", "error", err)
		return "", err
	}

	if configContent == "" {
		cd.logger.Error("no config found in database")
		return "", os.ErrNotExist
	}

	return configContent, nil
}

func main() {
	if len(os.Args) != 2 {
		slog.Error("usage: dump-config <db-file>")
		os.Exit(1)
	}

	dbPath := os.Args[1]

	dumper := NewConfigDumper(dbPath)
	if err := dumper.OpenDatabase(); err != nil {
		os.Exit(1)
	}
	defer dumper.pool.Close()

	configContent, err := dumper.DumpLatestConfig()
	if err != nil {
		os.Exit(1)
	}

	// Write config to stdout
	if _, err := os.Stdout.Write([]byte(configContent)); err != nil {
		dumper.logger.Error("failed to write config", "error", err)
		os.Exit(1)
	}
}
