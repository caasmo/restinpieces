package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/caasmo/restinpieces/config"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type AppCreator struct {
	dbfile        string
	migrationsDir string
	verbose       bool
	logger        *slog.Logger
	pool          *sqlitex.Pool
}

func NewAppCreator(dbfile string, verbose bool) *AppCreator {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}

	return &AppCreator{
		dbfile:        dbfile,
		migrationsDir: migrationsDir,
		verbose:       verbose,
		logger:        logger,
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
	
	// Read migration files from embedded FS
	migrations, err := fs.ReadDir(migrations.Schema(), ".")
	if err != nil {
		ac.logger.Error("failed to read embedded migrations", "error", err)
		return err
	}

	for _, migration := range migrations {
		if filepath.Ext(migration.Name()) != ".sql" {
			continue
		}

		sql, err := fs.ReadFile(migrations.Schema(), migration.Name())
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
	verbose := flag.Bool("v", false, "Enable verbose output")
	flag.Parse()

	creator := NewAppCreator(*dbfile, *migrationsDir, *verbose)

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
