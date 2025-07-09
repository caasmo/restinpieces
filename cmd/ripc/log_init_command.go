package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/migrations"
	"github.com/pelletier/go-toml/v2"
	"zombiezen.com/go/sqlite/sqlitex"
)

const defaultLogFilename = "logs.db"

func handleLogInitCommand(secureStore config.SecureStore, appDbPath string) {
	// Get log db path from config, or use default
	logDbPath, err := getLogDbPathFromConfig(secureStore, appDbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to determine log database path: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Initializing log database at: %s\n", logDbPath)

	// Connect to the log database (creates the file if it doesn't exist)
	pool, err := sqlitex.NewPool(logDbPath, sqlitex.PoolOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open/create log database at %s: %v\n", logDbPath, err)
		os.Exit(1)
	}
	defer func() {
		if err := pool.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing log database pool: %v\n", err)
		}
	}()

	// Apply the schema
	if err := runLogMigrations(pool); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to apply log schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Log database initialized successfully.")
}

func getLogDbPathFromConfig(secureStore config.SecureStore, appDbPath string) (string, error) {
	decryptedBytes, _, err := secureStore.Get(config.ScopeApplication, 0)
	if err != nil {
		// This can happen if the config was never saved, which is a valid scenario.
		// In this case, we fall back to the default.
		fmt.Println("Could not read configuration, using default log path.")
		return filepath.Join(filepath.Dir(appDbPath), defaultLogFilename), nil
	}

	var cfg config.Config
	if err := toml.Unmarshal(decryptedBytes, &cfg); err != nil {
		return "", fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Log.Batch.DbPath != "" {
		return cfg.Log.Batch.DbPath, nil
	}

	// Default path if not set in config
	return filepath.Join(filepath.Dir(appDbPath), defaultLogFilename), nil
}

func runLogMigrations(pool *sqlitex.Pool) error {
	conn, err := pool.Take(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get connection from pool: %w", err)
	}
	defer pool.Put(conn)

	schemaFS, err := fs.Sub(migrations.Schema(), "log")
	if err != nil {
		return fmt.Errorf("failed to access embedded log migrations: %w", err)
	}

	sqlBytes, err := fs.ReadFile(schemaFS, "logs.sql")
	if err != nil {
		return fmt.Errorf("failed to read embedded migration file logs.sql: %w", err)
	}

	fmt.Println("Applying log schema...")
	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		return fmt.Errorf("failed to execute migration file logs.sql: %w", err)
	}

	return nil
}
