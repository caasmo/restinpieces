package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

func handleAppCreateCommand(secureStore config.SecureStore, pool *sqlitex.Pool, dbPath string) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Check if database already exists
	if _, err := os.Stat(dbPath); err == nil {
		logger.Error("database file already exists", "file", dbPath)
		os.Exit(1)
	}

	// Run Migrations (Apply Schema)
	if err := runMigrations(logger, pool); err != nil {
		os.Exit(1) // Error logged in runMigrations
	}

	// Generate Default Config Struct
	defaultCfg, err := config.NewDefaultConfig(), nil
	if err != nil {
		logger.Error("failed to generate default config struct", "error", err)
		os.Exit(1)
	}

	// Marshal Config to TOML
	tomlBytes, err := toml.Marshal(defaultCfg)
	if err != nil {
		logger.Error("failed to marshal default config to TOML", "error", err)
		os.Exit(1)
	}

	// Save Encrypted Config into DB via SecureConfig
	if err := saveConfig(logger, secureStore, tomlBytes); err != nil {
		// Error logged in saveConfig
		os.Exit(1)
	}

	logger.Info("application database created and configured successfully", "db_file", dbPath)
}

func runMigrations(logger *slog.Logger, pool *sqlitex.Pool) error {
	conn, err := pool.Take(context.Background())
	if err != nil {
		logger.Error("failed to get connection from pool for migrations", "error", err)
		return err
	}
	defer pool.Put(conn)

	schemaFS := migrations.Schema()
	migrationFiles, err := fs.ReadDir(schemaFS, ".")
	if err != nil {
		logger.Error("failed to read embedded migrations", "error", err)
		return err
	}

	for _, migration := range migrationFiles {
		if filepath.Ext(migration.Name()) != ".sql" {
			continue
		}

		sqlBytes, err := fs.ReadFile(schemaFS, migration.Name())
		if err != nil {
			logger.Error("failed to read embedded migration",
				"file", migration.Name(),
				"error", err)
			return err
		}

		logger.Info("applying migration", "file", migration.Name())
		if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
			logger.Error("failed to execute migration",
				"file", migration.Name(),
				"error", err)
			return err
		}
	}
	return nil
}

func saveConfig(logger *slog.Logger, secureStore config.SecureStore, configData []byte) error {
	logger.Info("saving initial configuration using SecureStore")
	err := secureStore.Save(
		config.ScopeApplication,
		configData,
		"toml",
		"Initial default configuration",
	)
	if err != nil {
		logger.Error("failed to save initial config via SecureStore", "error", err)
		return fmt.Errorf("failed to save initial config: %w", err)
	}
	return nil
}
