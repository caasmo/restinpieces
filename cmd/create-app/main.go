package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type AppCreator struct {
	dbfile string
	logger *slog.Logger
	pool   *sqlitex.Pool
}

func (ac *AppCreator) generateEnvFile() ([]byte, error) {
	tmpl, err := template.New("env").Parse(string(config.EnvTemplate))
	if err != nil {
		return nil, fmt.Errorf("failed to parse env template: %w", err)
	}

	vars := struct {
		JWTAuthSecret          string
		JWTVerificationSecret  string
		JWTPasswordResetSecret string
		JWTEmailChangeSecret   string
	}{
		JWTAuthSecret:          crypto.RandomString(32, crypto.AlphanumericAlphabet),
		JWTVerificationSecret:  crypto.RandomString(32, crypto.AlphanumericAlphabet),
		JWTPasswordResetSecret: crypto.RandomString(32, crypto.AlphanumericAlphabet),
		JWTEmailChangeSecret:   crypto.RandomString(32, crypto.AlphanumericAlphabet),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, fmt.Errorf("failed to execute env template: %w", err)
	}

	return buf.Bytes(), nil
}

func (ac *AppCreator) CreateEnvFile(envPath string) error {
	if _, err := os.Stat(envPath); err == nil {
		ac.logger.Error(".env file already exists - remove it first if you want to recreate it")
		return os.ErrExist
	}

	envContent, err := ac.generateEnvFile()
	if err != nil {
		ac.logger.Error("failed to generate env file content", "error", err)
		return err
	}

	if err := os.WriteFile(envPath, envContent, 0644); err != nil {
		ac.logger.Error("failed to create env file", "path", envPath, "error", err)
		return err
	}

	ac.logger.Info("created .env file from template with auto-generated secrets")
	return nil
}

func NewAppCreator(dbfile string) *AppCreator {
	return &AppCreator{
		dbfile: dbfile,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

func (ac *AppCreator) CreateDatabase(dbPath string) error {
	if _, err := os.Stat(dbPath); err == nil {
		ac.logger.Error("database file already exists", "file", dbPath)
		return os.ErrExist
	}

	pool, err := sqlitex.NewPool(dbPath, sqlitex.PoolOptions{
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
				string(config.TomlExample),
				"toml",
				"Initial default configuration",
			},
		})
	return err
}

func main() {
	var (
		envFile string
		dbFile  string
		envSet  bool // Track if -env flag was set
		dbSet   bool // Track if -db flag was set
	)

	// Set defaults but track if flags were explicitly set
	flag.StringVar(&envFile, "env", ".env", "create .env file at specified path (default: .env)")
	flag.StringVar(&dbFile, "db", "app.db", "create database file at specified path (default: app.db)")
	flag.Parse()

	// Check which flags were explicitly set
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "env":
			envSet = true
		case "db":
			dbSet = true
		}
	})

	// Determine tasks based on which flags were set
	tasks := []string{}
	if envSet {
		tasks = append(tasks, "env")
	}
	if dbSet {
		tasks = append(tasks, "db")
	}
	if len(tasks) == 0 {
		// Default case - do both since no flags were set
		tasks = []string{"env", "db"}
	}

	creator := NewAppCreator()
	var poolClosed bool
	defer func() {
		if creator.pool != nil && !poolClosed {
			creator.pool.Close()
		}
	}()

	for _, task := range tasks {
		switch task {
		case "env":
			if err := creator.CreateEnvFile(envFile); err != nil {
				os.Exit(1)
			}
			creator.logger.Info("created env file", "path", envFile)

		case "db":
			creator.logger.Info("creating sqlite file", "path", dbFile)
			if err := creator.CreateDatabase(dbFile); err != nil {
				os.Exit(1)
			}
			poolClosed = true

			if err := creator.RunMigrations(); err != nil {
				os.Exit(1)
			}

			if err := creator.InsertConfig(); err != nil {
				creator.logger.Error("failed to insert config", "error", err)
				os.Exit(1)
			}
			creator.logger.Info("database created successfully", "file", dbFile)
		}
	}

	if len(tasks) > 1 {
		creator.logger.Info("application setup completed",
			"env_file", envFile,
			"db_file", dbFile)
	}
}
