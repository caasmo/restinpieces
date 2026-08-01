package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/migrations"
	"github.com/pelletier/go-toml/v2"
	"zombiezen.com/go/sqlite/sqlitex"
)

const defaultLogFilename = "logs.db"

var (
	ErrGetLogDbPath     = errors.New("failed to get log db path")
	ErrCreateLogDbPool  = errors.New("failed to create log db pool")
	ErrRunLogMigrations = errors.New("failed to run log migrations")
)

// handleLogInitCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleLogInitCommand(secureStore config.SecureStore, appDbPath string) {
	if err := logInit(os.Stdout, secureStore, appDbPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// logInit contains the testable core logic for initializing the log database.
func logInit(stdout io.Writer, secureStore config.SecureStore, appDbPath string) (err error) {
	// Get log db path from config, or use default
	logDbPath, usedDefault, err := getLogDbPathFromConfig(secureStore, appDbPath)
	if err != nil {
		return err // Already wrapped
	}
	if usedDefault {
		if _, err := fmt.Fprintln(stdout, "Could not read configuration, using default log path."); err != nil {
			return fmt.Errorf("%w: %w", ErrWriteOutput, err)
		}
	}

	if _, err := fmt.Fprintf(stdout, "Initializing log database at: %s\n", logDbPath); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	// Connect to the log database (creates the file if it doesn't exist)
	pool, err := sqlitex.NewPool(logDbPath, sqlitex.PoolOptions{})
	if err != nil {
		return fmt.Errorf("%w: failed to open/create log database at %s: %w", ErrCreateLogDbPool, logDbPath, err)
	}
	defer func() {
		if closeErr := pool.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("error closing log database pool: %w", closeErr)
		}
	}()

	// Apply the schema
	if err := runLogMigrations(stdout, pool); err != nil {
		return err // Already wrapped
	}

	if _, err := fmt.Fprintln(stdout, "Log database initialized successfully."); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	return nil
}

// getLogDbPathFromConfig determines the path for the log database.
// It returns the path, a boolean indicating if the default was used, and any error.
func getLogDbPathFromConfig(secureStore config.SecureStore, appDbPath string) (string, bool, error) {
	decryptedBytes, _, err := secureStore.Get(config.ScopeApplication, 0)
	if err != nil {
		// Fall back to the default path if config can't be read.
		return filepath.Join(filepath.Dir(appDbPath), defaultLogFilename), true, nil
	}

	var cfg config.Config
	if err := toml.Unmarshal(decryptedBytes, &cfg); err != nil {
		return "", false, fmt.Errorf("%w: failed to parse config: %w", ErrGetLogDbPath, err)
	}

	if cfg.Log.Batch.DbPath != "" {
		return cfg.Log.Batch.DbPath, false, nil
	}

	// Default path if not set in config
	return filepath.Join(filepath.Dir(appDbPath), defaultLogFilename), true, nil
}

// runLogMigrations applies the necessary SQL schema to the log database.
func runLogMigrations(stdout io.Writer, pool *sqlitex.Pool) error {
	conn, err := pool.Take(context.Background())
	if err != nil {
		return fmt.Errorf("%w: failed to get connection from pool: %w", ErrDbConnection, err)
	}
	defer pool.Put(conn)

	schemaFS, err := fs.Sub(migrations.Schema(), "log")
	if err != nil {
		return fmt.Errorf("%w: failed to access embedded log migrations: %w", ErrRunLogMigrations, err)
	}

	sqlBytes, err := fs.ReadFile(schemaFS, "logs.sql")
	if err != nil {
		return fmt.Errorf("%w: failed to read embedded migration file logs.sql: %w", ErrRunLogMigrations, err)
	}

	if _, err := fmt.Fprintln(stdout, "Applying log schema..."); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
		return fmt.Errorf("%w: failed to execute migration file logs.sql: %w", ErrRunLogMigrations, err)
	}

	return nil
}