package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	zdb "github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite/sqlitex"
)

type AppCreator struct {
	logger      *slog.Logger
	pool        *sqlitex.Pool
	secureStore config.SecureStore
}

func NewAppCreator() *AppCreator {
	return &AppCreator{
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

// CreateDatabasePool initializes the database pool.
func (ac *AppCreator) CreateDatabasePool(dbPath string) error {
	if _, err := os.Stat(dbPath); err == nil {
		ac.logger.Error("database file already exists", "file", dbPath)
		return os.ErrExist
	}

	// Use the library helper to create the pool, ensuring consistency
	pool, err := restinpieces.NewZombiezenPool(dbPath)
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
		ac.logger.Error("failed to get connection from pool for migrations", "error", err)
		return err
	}
	defer ac.pool.Put(conn)

	schemaFS := migrations.Schema()
	migrationFiles, err := fs.ReadDir(schemaFS, ".")
	if err != nil {
		ac.logger.Error("failed to read embedded migrations", "error", err)
		return err
	}

	for _, migration := range migrationFiles {
		if filepath.Ext(migration.Name()) != ".sql" {
			continue
		}

		sqlBytes, err := fs.ReadFile(schemaFS, migration.Name())
		if err != nil {
			ac.logger.Error("failed to read embedded migration",
				"file", migration.Name(),
				"error", err)
			return err
		}

		ac.logger.Info("applying migration", "file", migration.Name())
		if err := sqlitex.ExecuteScript(conn, string(sqlBytes), nil); err != nil {
			ac.logger.Error("failed to execute migration",
				"file", migration.Name(),
				"error", err)
			return err
		}
	}
	return nil
}

func (ac *AppCreator) generateDefaultConfig() (*config.Config, error) {
	return config.NewDefaultConfig(), nil
}

// SaveConfig uses the configured SecureStore implementation to save the config.
func (ac *AppCreator) SaveConfig(configData []byte) error {
	ac.logger.Info("saving initial configuration using SecureStore")
	err := ac.secureStore.Save(
		config.ScopeApplication,
		configData,
		"toml",
		"Initial default configuration",
	)
	if err != nil {
		ac.logger.Error("failed to save initial config via SecureStore", "error", err)
		return fmt.Errorf("failed to save initial config: %w", err)
	}
	return nil
}

func main() {
	dbPathFlag := flag.String("db", "", "Path to the SQLite database file to create (required)")
	ageKeyPathFlag := flag.String("age-key", "", "Path to the age identity (private key) file for encryption (required)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -db <database-path> -age-key <identity-file-path>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Creates a new SQLite database with an initial, encrypted configuration.\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *dbPathFlag == "" || *ageKeyPathFlag == "" {
		flag.Usage()
		os.Exit(1)
	}

	creator := NewAppCreator()

	// 1. Create Database Pool
	creator.logger.Info("creating sqlite database pool", "path", *dbPathFlag)
	if err := creator.CreateDatabasePool(*dbPathFlag); err != nil {
		os.Exit(1) // Error logged in CreateDatabasePool
	}
	defer func() {
		if creator.pool != nil {
			creator.logger.Info("closing database pool")
			if err := creator.pool.Close(); err != nil {
				creator.logger.Error("error closing database pool", "error", err)
			}
		}
	}()

	// 2. Instantiate DB implementation
	dbImpl, err := zdb.New(creator.pool)
	if err != nil {
		creator.logger.Error("failed to instantiate zombiezen db", "error", err)
		os.Exit(1)
	}

	// 3. Instantiate SecureStore
	secureStore, err := config.NewSecureStoreAge(dbImpl, *ageKeyPathFlag)
	if err != nil {
		creator.logger.Error("failed to instantiate secure store (age)", "error", err)
		os.Exit(1)
	}
	creator.secureStore = secureStore // Assign to creator

	// 4. Run Migrations (Apply Schema)
	if err := creator.RunMigrations(); err != nil {
		os.Exit(1) // Error logged in RunMigrations
	}

	// 5. Generate Default Config Struct
	defaultCfg, err := creator.generateDefaultConfig()
	if err != nil {
		creator.logger.Error("failed to generate default config struct", "error", err)
		os.Exit(1)
	}

	// 6. Marshal Config to TOML
	tomlBytes, err := toml.Marshal(defaultCfg)
	if err != nil {
		creator.logger.Error("failed to marshal default config to TOML", "error", err)
		os.Exit(1)
	}

	// 7. Save Encrypted Config into DB via SecureConfig
	if err := creator.SaveConfig(tomlBytes); err != nil {
		// Error logged in SaveConfig
		os.Exit(1)
	}

	creator.logger.Info("application database created and configured successfully", "db_file", *dbPathFlag)
}
