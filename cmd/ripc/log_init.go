package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
	"zombiezen.com/go/sqlite/sqlitex"
)

const defaultLogFilename = "logs.db"

var (
	ErrGetLogDbPath = errors.New("failed to get log db path")
	ErrApplyLogSQL  = errors.New("failed to apply log SQL")
)

// handleLogInitCommand is the command-level wrapper. It executes the core logic
// and returns any error to the caller.
func handleLogInitCommand(secureStore config.SecureStore, appDbPath string, ui UI) error {
	return logInit(ui, secureStore, appDbPath)
}

// logInit contains the testable core logic for initializing the log database.
func logInit(ui UI, secureStore config.SecureStore, appDbPath string) (err error) {
	// Get log db path from config, or use default
	logDbPath, usedDefault, err := getLogDbPathFromConfig(secureStore, appDbPath)
	if err != nil {
		return err // Already wrapped
	}
	if usedDefault {
		if _, err := fmt.Fprintln(ui.Err, "Could not read configuration, using default log path."); err != nil {
			return fmt.Errorf("%w: %w", ErrWriteOutput, err)
		}
	}

	if _, err := fmt.Fprintf(ui.Err, "Initializing log database at: %s\n", logDbPath); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	// Connect to the log database (creates the file if it doesn't exist)
	pool, err := sqlitex.NewPool(logDbPath, sqlitex.PoolOptions{})
	if err != nil {
		return fmt.Errorf("%w: failed to open/create log database at %s: %w", ErrDbConnection, logDbPath, err)
	}
	defer func() {
		if closeErr := pool.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("error closing log database pool: %w", closeErr)
		}
	}()

	// Apply the schema
	if err := applyLogSchema(ui, pool); err != nil {
		return err // Already wrapped
	}

	if _, err := fmt.Fprintln(ui.Err, "Log database initialized successfully."); err != nil {
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

func applyLogSchema(ui UI, pool *sqlitex.Pool) error {
	conn, err := pool.Take(context.Background())
	if err != nil {
		return fmt.Errorf("%w: failed to get connection from pool: %w", ErrDbConnection, err)
	}
	defer pool.Put(conn)

	if _, err := fmt.Fprintln(ui.Err, "Applying log schema..."); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	if err := applySQL(conn, "log"); err != nil {
		return fmt.Errorf("%w: failed to execute log sql: %w", ErrApplyLogSQL, err)
	}

	return nil
}
