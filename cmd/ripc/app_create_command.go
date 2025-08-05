package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/pelletier/go-toml/v2"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

var (
	ErrExecMigration = errors.New("failed to execute migration")
)

// handleAppCreateCommand is the command-level wrapper that executes the core app creation logic.
func handleAppCreateCommand(secureStore config.SecureStore, pool *sqlitex.Pool, dbPath string) {
	if err := createApplication(os.Stdout, secureStore, pool, dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// createApplication contains the testable core logic for creating and configuring the application.
func createApplication(stdout io.Writer, secureStore config.SecureStore, pool *sqlitex.Pool, dbPath string) error {
	// Run Migrations (Apply Schema)
	if err := runMigrations(stdout, pool); err != nil {
		return err // Error is already wrapped by runMigrations
	}

	// Generate Default Config Struct
	defaultCfg := config.NewDefaultConfig()

	// Marshal Config to TOML
	tomlBytes, err := toml.Marshal(defaultCfg)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal default config to TOML: %w", ErrConfigMarshal, err)
	}

	// Save Encrypted Config into DB via SecureConfig
	if err := saveConfig(stdout, secureStore, tomlBytes); err != nil {
		return err // Error is already wrapped by saveConfig
	}

	if _, err := fmt.Fprintf(stdout, "Application database created and configured successfully: %s\n", dbPath); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}

func runMigrations(stdout io.Writer, pool *sqlitex.Pool) error {
	conn, err := pool.Take(context.Background())
	if err != nil {
		return fmt.Errorf("%w: for migrations: %w", ErrDbConnection, err)
	}
	defer pool.Put(conn)

	if _, err := fmt.Fprintln(stdout, "Applying migrations..."); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	schemaFS := migrations.Schema()
	if err := zombiezen.ApplyMigrations(conn, schemaFS); err != nil {
		return fmt.Errorf("%w: migration process failed: %w", ErrExecMigration, err)
	}

	return nil
}

func saveConfig(stdout io.Writer, secureStore config.SecureStore, configData []byte) error {
	if _, err := fmt.Fprintln(stdout, "Saving initial configuration..."); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	err := secureStore.Save(
		config.ScopeApplication,
		configData,
		"toml",
		"Initial default configuration",
	)
	if err != nil {
		return fmt.Errorf("%w: failed to save initial config: %w", ErrSecureStoreSave, err)
	}
	return nil
}
