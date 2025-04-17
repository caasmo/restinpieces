package main

import (
	// Keep other imports the same
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	// No need for "bufio" or "strings" for key extraction anymore
	"time"

	"filippo.io/age"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// ConfigInserter struct and NewConfigInserter function remain the same
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

// OpenDatabase function remains the same
func (ci *ConfigInserter) OpenDatabase() error {
	pool, err := sqlitex.NewPool(ci.dbfile, sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadWrite | sqlite.OpenCreate,
		PoolSize: runtime.NumCPU(),
	})
	if err != nil {
		ci.logger.Error("failed to open database", "error", err)
		return err
	}
	ci.pool = pool
	return nil
}

// --- Remove extractPublicKeyFromIdentity function ---

func (ci *ConfigInserter) InsertConfig(tomlPath, ageIdentityPath string) error {
	// Read config file
	configData, err := os.ReadFile(tomlPath)
	if err != nil {
		ci.logger.Error("failed to read config file", "path", tomlPath, "error", err)
		return err
	}

	// Read age identity file
	ageIdentityData, err := os.ReadFile(ageIdentityPath)
	if err != nil {
		ci.logger.Error("failed to read age identity file", "path", ageIdentityPath, "error", err)
		return fmt.Errorf("failed to read age identity file '%s': %w", ageIdentityPath, err)
	}

	// --- Start Modification (Use ParseIdentities) ---

	// Parse the identity file content
	identities, err := age.ParseIdentities(bytes.NewReader(ageIdentityData))
	if err != nil {
		return fmt.Errorf("failed to parse age identity file '%s': %w", ageIdentityPath, err)
	}

	if len(identities) == 0 {
		// This case should theoretically be caught by the error above, but check defensively
		ci.logger.Error("no age identities found in file", "path", ageIdentityPath)
		return fmt.Errorf("no age identities found in file '%s'", ageIdentityPath)
	}

	// For this script's purpose, we only need one identity to get the public key.
	// Use the first identity found.
	identity := identities[0]

	// Get the corresponding Recipient (public key) from the Identity
	var recipient age.Recipient
	switch id := identity.(type) {
	case *age.X25519Identity:
		recipient = id.Recipient()
	default:
		// For SSH identities, we can't directly get a recipient
		ci.logger.Error("unsupported age identity type - must be X25519",
			"path", ageIdentityPath,
			"type", fmt.Sprintf("%T", identity))
		return fmt.Errorf("unsupported age identity type '%T' - must be X25519", identity)
	}

	// --- End Modification ---

	// Encrypt the config data using the derived recipient
	encryptedOutput := &bytes.Buffer{}
	// Note: Pass the single recipient directly, no need for recipients... syntax here
	encryptWriter, err := age.Encrypt(encryptedOutput, recipient)
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

	// --- Database insertion logic remains the same ---
	conn, err := ci.pool.Take(context.Background())
	if err != nil {
		ci.logger.Error("failed to get database connection", "error", err)
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
				encryptedData,
				"toml",
				description,
				now,
			},
		})

	if err != nil {
		ci.logger.Error("failed to insert config", "error", err)
		return fmt.Errorf("database insert failed: %w", err)
	}

	ci.logger.Info("Successfully inserted encrypted config", "toml_file", tomlPath, "identity_file", ageIdentityPath)
	return nil
}

// main function remains the same
func main() {
	ageIdentityPathFlag := flag.String("age-key", "", "Path to the age identity file (containing private key 'AGE-SECRET-KEY-1...') (required)")
	dbPathFlag := flag.String("db", "", "Path to the SQLite database file (required)")
	tomlPathFlag := flag.String("toml", "", "Path to the TOML config file to insert (required)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -age-key <identity-file> -db <db-file> -toml <toml-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Encrypts a TOML file using the public key derived from an age identity file and inserts it into a SQLite DB.\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *ageIdentityPathFlag == "" || *dbPathFlag == "" || *tomlPathFlag == "" {
		flag.Usage()
		os.Exit(1)
	}

	inserter := NewConfigInserter(*dbPathFlag)
	if err := inserter.OpenDatabase(); err != nil {
		os.Exit(1)
	}
	defer inserter.pool.Close()

	if err := inserter.InsertConfig(*tomlPathFlag, *ageIdentityPathFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
