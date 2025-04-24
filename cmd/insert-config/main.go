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

	"github.com/caasmo/restinpieces/db"         // Import db package for scope constant and interface
	dbz "github.com/caasmo/restinpieces/db/zombiezen" // Import zombiezen implementation
)

// --- ConfigInserter struct and methods removed ---

// insertConfig encapsulates the logic previously in ConfigInserter.InsertConfig
func insertConfig(dbConn db.DbConfig, logger *slog.Logger, tomlPath, ageIdentityPath, scope string) error {
	// Read config file
	configData, err := os.ReadFile(tomlPath)
	if err != nil {
		logger.Error("failed to read config file", "path", tomlPath, "error", err)
		return err
	}

	// Read age identity file
	ageIdentityData, err := os.ReadFile(ageIdentityPath)
	if err != nil {
		logger.Error("failed to read age identity file", "path", ageIdentityPath, "error", err)
		return fmt.Errorf("failed to read age identity file '%s': %w", ageIdentityPath, err)
	}

	// --- Encryption Logic (remains largely the same) ---

	// Parse the identity file content
	identities, err := age.ParseIdentities(bytes.NewReader(ageIdentityData))
	if err != nil {
		logger.Error("failed to parse age identity file", "path", ageIdentityPath, "error", err)
		return fmt.Errorf("failed to parse age identity file '%s': %w", ageIdentityPath, err)
	}

	if len(identities) == 0 {
		logger.Error("no age identities found in file", "path", ageIdentityPath)
		return fmt.Errorf("no age identities found in file '%s'", ageIdentityPath)
	}

	// Use the first identity found.
	identity := identities[0]

	// Get the corresponding Recipient (public key) from the Identity
	var recipient age.Recipient
	switch id := identity.(type) {
	case *age.X25519Identity:
		recipient = id.Recipient()
	default:
		logger.Error("unsupported age identity type - must be X25519",
			"path", ageIdentityPath,
			"type", fmt.Sprintf("%T", identity))
		return fmt.Errorf("unsupported age identity type '%T' - must be X25519", identity)
	}

	// Encrypt the config data
	encryptedOutput := &bytes.Buffer{}
	encryptWriter, err := age.Encrypt(encryptedOutput, recipient)
	if err != nil {
		logger.Error("failed to create age encryption writer", "error", err)
		return fmt.Errorf("failed to create age encryption writer: %w", err)
	}
	if _, err := io.Copy(encryptWriter, bytes.NewReader(configData)); err != nil {
		logger.Error("failed to write data to age encryption writer", "error", err)
		return fmt.Errorf("failed to write data to age encryption writer: %w", err)
	}
	if err := encryptWriter.Close(); err != nil {
		logger.Error("failed to close age encryption writer", "error", err)
		return fmt.Errorf("failed to close age encryption writer: %w", err)
	}
	encryptedData := encryptedOutput.Bytes()

	// --- Call the DbConfig InsertConfig method ---
	description := "Inserted from file: " + filepath.Base(tomlPath)
	err = dbConn.InsertConfig(scope, encryptedData, "toml", description)
	if err != nil {
		// The db method already formats the error, just log and return
		logger.Error("failed to insert config via db method", "scope", scope, "error", err)
		return err // Return the error directly from dbConn.InsertConfig
	}

	logger.Info("Successfully inserted encrypted config", "scope", scope, "toml_file", tomlPath, "identity_file", ageIdentityPath)
	return nil
}

func main() {
	// --- Setup Logger ---
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// --- Flag Parsing (remains the same) ---
	ageIdentityPathFlag := flag.String("age-key", "", "Path to the age identity file (containing private key 'AGE-SECRET-KEY-1...') (required)")
	dbPathFlag := flag.String("db", "", "Path to the SQLite database file (required)")
	tomlPathFlag := flag.String("toml", "", "Path to the TOML config file to insert (required)")
	scopeFlag := flag.String("scope", db.ConfigScopeApplication, "Scope for the configuration (e.g., 'application', 'plugin_x')")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -age-key <identity-file> -db <db-file> -toml <toml-file> [-scope <scope>]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Encrypts a TOML file using the public key derived from an age identity file and inserts it into a SQLite DB under a specific scope.\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *ageIdentityPathFlag == "" || *dbPathFlag == "" || *tomlPathFlag == "" {
		flag.Usage()
		flag.Usage()
		os.Exit(1)
	}

	// --- Database Setup using zombiezen.New ---
	dbConn, err := dbz.New(*dbPathFlag) // Use zombiezen.New directly
	if err != nil {
		logger.Error("failed to open database pool", "db_path", *dbPathFlag, "error", err)
		os.Exit(1)
	}
	defer dbConn.Close() // Close the pool on exit

	// --- Call the refactored insertConfig function ---
	if err := insertConfig(dbConn, logger, *tomlPathFlag, *ageIdentityPathFlag, *scopeFlag); err != nil {
		// Error is already logged within insertConfig or the db method
		// fmt.Fprintf(os.Stderr, "Error: %v\n", err) // Keep stderr for script compatibility if needed
		os.Exit(1) // Exit with error status
	}

	// Success message is logged within insertConfig
}
