package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"

	"filippo.io/age"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type ConfigDumper struct {
	dbfile string
	logger *slog.Logger
	pool   *sqlitex.Pool
}

func NewConfigDumper(dbfile string) *ConfigDumper {
	return &ConfigDumper{
		dbfile: dbfile,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

func (cd *ConfigDumper) OpenDatabase() error {
	pool, err := sqlitex.NewPool(cd.dbfile, sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadWrite,
		PoolSize: runtime.NumCPU(),
	})
	if err != nil {
		cd.logger.Error("failed to open database", "error", err)
		return err
	}
	cd.pool = pool
	return nil
}

func (cd *ConfigDumper) DecryptConfig(encryptedData []byte, ageKeyPath string) ([]byte, error) {
	keyContent, err := os.ReadFile(ageKeyPath)
	if err != nil {
		cd.logger.Error("failed to read age key file", "path", ageKeyPath, "error", err)
		return nil, fmt.Errorf("failed to read age key file '%s': %w", ageKeyPath, err)
	}

	identities, err := age.ParseIdentities(bytes.NewReader(keyContent))
	if err != nil {
		cd.logger.Error("failed to parse age identities", "path", ageKeyPath, "error", err)
		return nil, fmt.Errorf("failed to parse age identities from key file '%s': %w", ageKeyPath, err)
	}
	if len(identities) == 0 {
		cd.logger.Error("no age identities found in key file", "path", ageKeyPath)
		return nil, fmt.Errorf("no age identities found in key file '%s'", ageKeyPath)
	}

	encryptedDataReader := bytes.NewReader(encryptedData)
	decryptedDataReader, err := age.Decrypt(encryptedDataReader, identities...)
	if err != nil {
		cd.logger.Error("failed to decrypt configuration data", "error", err)
		return nil, fmt.Errorf("failed to decrypt configuration data: %w", err)
	}

	return io.ReadAll(decryptedDataReader)
}

func (cd *ConfigDumper) GetLatestEncryptedConfig() ([]byte, error) {
	conn, err := cd.pool.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer cd.pool.Put(conn)

	var encryptedData []byte
	err = sqlitex.Execute(conn,
		`SELECT content FROM app_config 
		ORDER BY created_at DESC 
		LIMIT 1;`,
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				// Get a reader for the blob column (index 0)
				reader := stmt.ColumnReader(0)
				// Read all data from the reader
				var err error
				encryptedData, err = io.ReadAll(reader)
				return err // Return any error from io.ReadAll
			},
		})

	if err != nil {
		cd.logger.Error("failed to query config", "error", err)
		return nil, err
	}

	if len(encryptedData) == 0 {
		cd.logger.Error("no config found in database")
		return nil, os.ErrNotExist
	}

	return encryptedData, nil
}

func main() {
	// --- Setup Logger ---
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// --- Flag Parsing ---
	outputFileFlag := flag.String("output", "", "Output file path (writes to stdout if empty)")
	flag.StringVar(outputFileFlag, "o", "", "Output file path (shorthand)") // Link shorthand to the same variable
	ageKeyPathFlag := flag.String("age-key", "", "Path to the age identity file (private key 'AGE-SECRET-KEY-1...') (required)")
	scopeFlag := flag.String("scope", config.ScopeApplication, "Scope for the configuration (e.g., 'application', 'plugin_x')")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -age-key <identity-file> <db-file> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Retrieves the latest configuration for a scope, decrypts it using an age identity, and writes it to output.\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  <db-file>          Path to the SQLite database file (required)\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// --- Validate Arguments and Flags ---
	if *ageKeyPathFlag == "" {
		logger.Error("missing required flag: -age-key")
		flag.Usage()
		os.Exit(1)
	}
	if flag.NArg() != 1 {
		logger.Error("missing required argument: <db-file>")
		flag.Usage()
		os.Exit(1)
	}
	dbPath := flag.Arg(0)

	// --- Database Setup ---
	dbImpl, err := dbz.New(dbPath)
	if err != nil {
		logger.Error("failed to instantiate zombiezen db", "db_path", dbPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		logger.Info("closing database connection")
		if err := dbImpl.Close(); err != nil {
			logger.Error("error closing database connection", "error", err)
		}
	}()

	// --- Instantiate SecureConfig ---
	secureCfg, err := config.NewSecureConfigAge(dbImpl, *ageKeyPathFlag, logger)
	if err != nil {
		logger.Error("failed to instantiate secure config (age)", "age_key_path", *ageKeyPathFlag, "error", err)
		os.Exit(1)
	}

	// --- Get Latest Config using SecureConfig ---
	logger.Info("retrieving latest configuration", "scope", *scopeFlag)
	decryptedData, err := secureCfg.Latest(*scopeFlag)
	if err != nil {
		// SecureConfig.Latest logs specifics, just log the failure here
		logger.Error("failed to retrieve latest config via SecureConfig", "scope", *scopeFlag, "error", err)
		// fmt.Fprintf(os.Stderr, "Error: %v\n", err) // Keep stderr for script compatibility if needed
		os.Exit(1)
	}

	// --- Write Output ---
	if *outputFileFlag != "" {
		err := os.WriteFile(*outputFileFlag, decryptedData, 0644)
		if err != nil {
			logger.Error("failed to write config file",
				"path", *outputFileFlag,
				"error", err)
			os.Exit(1)
		}
		logger.Info("config written to file", "path", *outputFileFlag, "scope", *scopeFlag)
	} else {
		// Write to stdout
		if _, err := os.Stdout.Write(decryptedData); err != nil {
			logger.Error("failed to write config to stdout", "scope", *scopeFlag, "error", err)
			os.Exit(1)
		}
		// Optionally log success to stderr if writing to stdout
		logger.Info("config written to stdout", "scope", *scopeFlag)
	}
}
