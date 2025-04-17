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
	var (
		outputFile   string
		ageKeyPath   string
	)

	flag.StringVar(&outputFile, "output", "", "output TOML file path")
	flag.StringVar(&outputFile, "o", "", "output TOML file path (shorthand)")
	flag.StringVar(&ageKeyPath, "age-key", "", "path to age identity file (required)")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 || ageKeyPath == "" {
		slog.Error("usage: dump-config -age-key <key-file> [-o|--output <file>] <db-file>")
		os.Exit(1)
	}

	dbPath := args[0]
	dumper := NewConfigDumper(dbPath)
	if err := dumper.OpenDatabase(); err != nil {
		os.Exit(1)
	}
	defer dumper.pool.Close()

	encryptedData, err := dumper.GetLatestEncryptedConfig()
	if err != nil {
		os.Exit(1)
	}

	decryptedData, err := dumper.DecryptConfig(encryptedData, ageKeyPath)
	if err != nil {
		os.Exit(1)
	}

	// Write to file if output specified, otherwise stdout
	if outputFile != "" {
		err := os.WriteFile(outputFile, decryptedData, 0644)
		if err != nil {
			dumper.logger.Error("failed to write config file",
				"path", outputFile,
				"error", err)
			os.Exit(1)
		}
		dumper.logger.Info("config written to file", "path", outputFile)
	} else {
		if _, err := os.Stdout.Write(decryptedData); err != nil {
			dumper.logger.Error("failed to write config", "error", err)
			os.Exit(1)
		}
	}
}
