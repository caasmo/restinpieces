package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

func handleAppCreateCommand(secureStore config.SecureStore, pool *sqlitex.Pool, dbPath string) {
	// Run Migrations (Apply Schema)
	if err := runMigrations(pool); err != nil {
		// Error already printed by runMigrations
		os.Exit(1)
	}

	// Generate Default Config Struct
	defaultCfg := config.NewDefaultConfig()

	// Marshal Config to TOML
	tomlBytes, err := toml.Marshal(defaultCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal default config to TOML: %v\n", err)
		os.Exit(1)
	}

	// Save Encrypted Config into DB via SecureConfig
	if err := saveConfig(secureStore, tomlBytes); err != nil {
		// Error already printed by saveConfig
		os.Exit(1)
	}

	fmt.Printf("Application database created and configured successfully: %s\n", dbPath)
}

func runMigrations(pool *sqlitex.Pool) error {
	conn, err := pool.Take(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get connection from pool for migrations: %v\n", err)
		return err
	}
	defer pool.Put(conn)

	schemaFS := migrations.Schema()
	migrationFiles, err := fs.ReadDir(schemaFS, ".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read embedded migrations: %v\n", err)
		return err
	}

	for _, migration := range migrationFiles {
		if filepath.Ext(migration.Name()) != ".sql" {
			continue
		}

		sqlBytes, err := fs.ReadFile(schemaFS, migration.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read embedded migration file %s: %v\n", migration.Name(), err)
			return err
		}

		fmt.Printf("Applying migration: %s\n", migration.Name())
		if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to execute migration file %s: %v\n", migration.Name(), err)
			return err
		}
	}
	return nil
}

func saveConfig(secureStore config.SecureStore, configData []byte) error {
	fmt.Println("Saving initial configuration...")
	err := secureStore.Save(
		config.ScopeApplication,
		configData,
		"toml",
		"Initial default configuration",
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save initial config via SecureStore: %v\n", err)
		return fmt.Errorf("failed to save initial config: %w", err)
	}
	return nil
}
