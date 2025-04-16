package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"filippo.io/age"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

type ConfigInserter struct {
	dbfile string
	logger *slog.Logger
	pool   *sqlitex.Pool
}

func NewConfigInserter(dbfile string) *ConfigInserter {
	return &ConfigInserter{
		dbfile: dbfile,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

func (ci *ConfigInserter) OpenDatabase() error {
	pool, err := sqlitex.NewPool(ci.dbfile, sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadWrite,
		PoolSize: runtime.NumCPU(),
	})
	if err != nil {
		ci.logger.Error("failed to open database", "error", err)
		return err
	}
	ci.pool = pool
	return nil
}

func (ci *ConfigInserter) InsertConfig(tomlPath, ageKeyPath string) error {
	// Read config file
	configData, err := os.ReadFile(tomlPath)
	if err != nil {
		ci.logger.Error("failed to read config file", "path", tomlPath, "error", err)
		return err
	}

	// Read age public key file
	ageKeyData, err := os.ReadFile(ageKeyPath)
	if err != nil {
		ci.logger.Error("failed to read age key file", "path", ageKeyPath, "error", err)
		return fmt.Errorf("failed to read age key file '%s': %w", ageKeyPath, err)
	}

	// Parse age recipient (public key)
	recipients, err := age.ParseRecipients(bytes.NewReader(ageKeyData))
	if err != nil {
		ci.logger.Error("failed to parse age recipients", "path", ageKeyPath, "error", err)
		return fmt.Errorf("failed to parse age recipients from key file '%s': %w", ageKeyPath, err)
	}
	if len(recipients) == 0 {
		ci.logger.Error("no age recipients found in key file", "path", ageKeyPath)
		return fmt.Errorf("no age recipients found in key file '%s'", ageKeyPath)
	}

	// Encrypt the config data
	encryptedOutput := &bytes.Buffer{}
	encryptWriter, err := age.Encrypt(encryptedOutput, recipients...)
	if err != nil {
		ci.logger.Error("failed to create age encryption writer", "error", err)
		return fmt.Errorf("failed to create age encryption writer: %w", err)
	}
	if _, err := io.Copy(encryptWriter, bytes.NewReader(configData)); err != nil {
		ci.logger.Error("failed to write data to age encryption writer", "error", err)
		return fmt.Errorf("failed to write data to age encryption writer: %w", err)
	}
	if err := encryptWriter.Close(); err != nil {
		ci.logger.Error("failed to close age encryption writer", "error", err)
		return fmt.Errorf("failed to close age encryption writer: %w", err)
	}
	encryptedData := encryptedOutput.Bytes()

	conn, err := ci.pool.Take(context.Background())
	if err != nil {
		return err
	}
	defer ci.pool.Put(conn)

	now := time.Now().UTC().Format(time.RFC3339)
	description := "Inserted from file: " + filepath.Base(tomlPath)

	err = sqlitex.Execute(conn,
		`INSERT INTO app_config (
			content, 
			format,
			description,
			created_at
		) VALUES (?, ?, ?, ?)`,
		&sqlitex.ExecOptions{
			Args: []interface{}{
				encryptedData, // content (now encrypted blob)
				"toml",        // format (still TOML before encryption)
				description,   // description
				now,           // created_at
			},
		})

	if err != nil {
		ci.logger.Error("failed to insert config", "error", err)
		return err
	}

	ci.logger.Info("successfully inserted config")
	return nil
}

func main() {
	// Define flags
	ageKeyPathFlag := flag.String("age-key", "", "Path to the age public key file (required)")
	dbPathFlag := flag.String("db", "", "Path to the SQLite database file (required)")
	tomlPathFlag := flag.String("toml", "", "Path to the TOML config file to insert (required)")

	// Set custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -age-key <key-file> -db <db-file> -toml <toml-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	// Parse flags
	flag.Parse()

	// Validate required flags
	if *ageKeyPathFlag == "" || *dbPathFlag == "" || *tomlPathFlag == "" {
		flag.Usage()
		os.Exit(1)
	}

	inserter := NewConfigInserter(*dbPathFlag)
	if err := inserter.OpenDatabase(); err != nil {
		os.Exit(1) // Error already logged
	}
	defer inserter.pool.Close()

	if err := inserter.InsertConfig(*tomlPathFlag, *ageKeyPathFlag); err != nil {
		os.Exit(1) // Error already logged
	}
}
