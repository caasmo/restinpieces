package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/migrations"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type AppCreator struct {
	logger *slog.Logger
	conn   *sqlite.Conn
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

// CreateConfigTomlFile creates a TOML configuration file from the embedded example.
func (ac *AppCreator) CreateConfigTomlFile(tomlPath string) error {
	if _, err := os.Stat(tomlPath); err == nil {
		ac.logger.Error("config.toml file already exists - remove it first if you want to recreate it", "path", tomlPath)
		return os.ErrExist
	}

	// Use the embedded config.TomlExample
	if err := os.WriteFile(tomlPath, config.TomlExample, 0644); err != nil {
		ac.logger.Error("failed to create config.toml file", "path", tomlPath, "error", err)
		return err
	}

	ac.logger.Info("created config.toml file from example", "path", tomlPath)
	return nil
}

func NewAppCreator() *AppCreator {
	return &AppCreator{
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

	conn, err := sqlite.OpenConn(dbPath, sqlite.OpenReadWrite|sqlite.OpenCreate)
	if err != nil {
		ac.logger.Error("failed to open database", "error", err)
		return err
	}
	ac.conn = conn
	return nil
}

func (ac *AppCreator) RunMigrations() error {
	schemaFS := migrations.Schema()
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
		if err := sqlitex.ExecuteScript(ac.conn, string(sql), &sqlitex.ExecOptions{
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
	ac.logger.Info("inserting default configuration")
	err := sqlitex.Execute(ac.conn,
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
		envFile  string
		dbFile   string
		tomlFile string // Variable for the TOML file path
		envSet   bool   // Track if -env flag was set
		dbSet    bool   // Track if -db flag was set
		tomlSet  bool   // Track if -toml flag was set
	)

	// Set defaults but track if flags were explicitly set
	flag.StringVar(&envFile, "env", ".env", "create .env file at specified path (default: .env)")
	flag.StringVar(&dbFile, "db", "app.db", "create database file at specified path (default: app.db)")
	flag.StringVar(&tomlFile, "toml", "config.toml", "create config.toml file at specified path (default: config.toml)") // New flag
	flag.Parse()

	// Check which flags were explicitly set
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "env":
			envSet = true
		case "db":
			dbSet = true
		case "toml": // Track the new flag
			tomlSet = true
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
	if tomlSet { // Add toml task if flag is set
		tasks = append(tasks, "toml")
	}
	if len(tasks) == 0 {
		// Default case - only do env and db if no flags were set
		tasks = []string{"env", "db"}
	}

	creator := NewAppCreator()
	defer func() {
		if creator.conn != nil {
			creator.conn.Close()
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

			if err := creator.RunMigrations(); err != nil {
				os.Exit(1)
			}

			if err := creator.InsertConfig(); err != nil {
				creator.logger.Error("failed to insert config", "error", err)
				os.Exit(1)
			}
			creator.logger.Info("database created successfully", "file", dbFile)

		case "toml": // Add case for the new task
			if err := creator.CreateConfigTomlFile(tomlFile); err != nil {
				os.Exit(1)
			}
			// Log message is already inside CreateConfigTomlFile
		}
	}

	// Log completion based on actual tasks performed
	logArgs := []any{}
	if envSet || (!envSet && !dbSet && !tomlSet) { // Log env if explicitly set or default run
		logArgs = append(logArgs, "env_file", envFile)
	}
	if dbSet || (!envSet && !dbSet && !tomlSet) { // Log db if explicitly set or default run
		logArgs = append(logArgs, "db_file", dbFile)
	}
	if tomlSet { // Log toml only if explicitly set
		logArgs = append(logArgs, "toml_file", tomlFile)
	}

	if len(logArgs) > 0 {
		creator.logger.Info("application setup task(s) completed", logArgs...)
	} else {
		// This case should ideally not happen with the current logic, but good to handle
		creator.logger.Info("no application setup tasks performed")
	}
}
