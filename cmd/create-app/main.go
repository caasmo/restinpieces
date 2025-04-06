package main

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func main() {
	// Parse command line flags
	dbfile := flag.String("dbfile", "app.db", "SQLite database file to create")
	migrationsDir := flag.String("migrations", "migrations", "Directory containing migration SQL files")
	verbose := flag.Bool("v", false, "Enable verbose output")
	flag.Parse()

	// Set up logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if *verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}

	// Create database file if it doesn't exist
	if _, err := os.Stat(*dbfile); err == nil {
		logger.Error("database file already exists", "file", *dbfile)
		os.Exit(1)
	}

	// Open database connection
	conn, err := sqlite.OpenConn(*dbfile, sqlite.OpenReadWrite|sqlite.OpenCreate)
	if err != nil {
		logger.Error("failed to create database", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Read migration files
	migrations, err := os.ReadDir(*migrationsDir)
	if err != nil {
		logger.Error("failed to read migrations directory", "error", err)
		os.Exit(1)
	}

	// Execute migrations in order
	for _, migration := range migrations {
		if filepath.Ext(migration.Name()) != ".sql" {
			continue
		}

		path := filepath.Join(*migrationsDir, migration.Name())
		sql, err := os.ReadFile(path)
		if err != nil {
			logger.Error("failed to read migration file", "file", path, "error", err)
			os.Exit(1)
		}

		logger.Info("applying migration", "file", migration.Name())
		if err := sqlitex.ExecuteScript(conn, string(sql), &sqlitex.ExecOptions{
			Args: nil,
		}); err != nil {
			logger.Error("failed to execute migration", 
				"file", migration.Name(), 
				"error", err)
			os.Exit(1)
		}
	}

	logger.Info("database created successfully", "file", *dbfile)
}
