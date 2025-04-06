package main

import (
	"context"
	"flag"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type AppCreator struct {
	dbfile string
	logger *slog.Logger
	pool   *sqlitex.Pool
}

func NewAppCreator(dbfile string) *AppCreator {
	return &AppCreator{
		dbfile: dbfile,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

func (ac *AppCreator) CreateDatabase() error {
	if _, err := os.Stat(ac.dbfile); err == nil {
		ac.logger.Error("database file already exists", "file", ac.dbfile)
		return os.ErrExist
	}

	pool, err := sqlitex.NewPool(ac.dbfile, sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadWrite | sqlite.OpenCreate,
		PoolSize: runtime.NumCPU(),
	})
	if err != nil {
		ac.logger.Error("failed to create database pool", "error", err)
		return err
	}
	ac.pool = pool
	return nil
}

func (ac *AppCreator) RunMigrations() error {
	conn, err := ac.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer ac.pool.Put(conn)
	
	// Get embedded schema filesystem
	schemaFS := migrations.Schema()

	// Read migration files from embedded FS
	migrations, err := fs.ReadDir(schemaFS, ".")
	if err != nil {
		ac.logger.Error("failed to read embedded migrations", "error", err)
		return err
	}

	for _, migration := range migrations {
		if filepath.Ext(migration.Name()) != ".sql" {
			continue
		}

		sql, err := fs.ReadFile(schemaFS, migration.Name())
		if err != nil {
			ac.logger.Error("failed to read embedded migration", 
				"file", migration.Name(), 
				"error", err)
			return err
		}

		ac.logger.Info("applying migration", "file", migration.Name())
		if err := sqlitex.ExecuteScript(conn, string(sql), &sqlitex.ExecOptions{
			Args: nil,
		}); err != nil {
			ac.logger.Error("failed to execute migration", 
				"file", migration.Name(), 
				"error", err)
			return err
		}
	}
	return nil
}

func (ac *AppCreator) InsertConfig() error {
	conn, err := ac.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer ac.pool.Put(conn)
	ac.logger.Info("inserting default configuration")
	err = sqlitex.Execute(conn,
		`INSERT INTO app_config (content, format, description)
		VALUES (?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []interface{}{
				string(config.DefaultConfigToml),
				"toml",
				"Initial default configuration",
			},
		})
	return err
}

func main() {
	dbfile := flag.String("dbfile", "app.db", "SQLite database file to create")
	createEnv := flag.Bool("env", false, "Create .env file from example")
	flag.Parse()

	if *createEnv {
		if _, err := os.Stat(".env"); err == nil {
			slog.Error(".env file already exists")
			os.Exit(1)
		}
		if err := os.WriteFile(".env", config.DefaultEnvExample, 0644); err != nil {
			slog.Error("failed to create .env file", "error", err)
			os.Exit(1)
		}
		slog.Info("created .env file from example")
		os.Exit(0)
	}

	creator := NewAppCreator(*dbfile)

	if err := creator.CreateDatabase(); err != nil {
		os.Exit(1)
	}
	defer creator.pool.Close()

	if err := creator.RunMigrations(); err != nil {
		os.Exit(1)
	}

	if err := creator.InsertConfig(); err != nil {
		creator.logger.Error("failed to insert config", "error", err)
		os.Exit(1)
	}

	creator.logger.Info("database created successfully", "file", *dbfile)
}
