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
	"github.com/pelletier/go-toml/v2"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/caasmo/restinpieces/config"
)

type CertInserter struct {
	dbfile string
	logger *slog.Logger
	pool   *sqlitex.Pool
}

func NewCertInserter(dbfile string) *CertInserter {
	return &CertInserter{
		dbfile: dbfile,
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}
}

func (ci *CertInserter) OpenDatabase() error {
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

// getLatestEncryptedConfig fetches the most recent config blob from the database.
func (ci *CertInserter) getLatestEncryptedConfig(ctx context.Context) ([]byte, error) {
	conn, err := ci.pool.Take(ctx)
	if err != nil {
		ci.logger.Error("failed to get database connection", "error", err)
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}
	defer ci.pool.Put(conn)

	var encryptedData []byte
	err = sqlitex.Execute(conn, `SELECT content FROM app_config ORDER BY id DESC LIMIT 1`, &sqlitex.ExecOptions{
		ResultFunc: func(stmt *sqlite.Stmt) error {
			encryptedData = stmt.ColumnBytes(0)
			return nil
		},
	})
	if err != nil {
		ci.logger.Error("failed to query latest config", "error", err)
		return nil, fmt.Errorf("failed to query latest config: %w", err)
	}
	if len(encryptedData) == 0 {
		ci.logger.Error("no configuration found in the database")
		return nil, fmt.Errorf("no configuration found in the database")
	}
	return encryptedData, nil
}

// decryptData decrypts data using the provided age identity file.
func (ci *CertInserter) decryptData(encryptedData []byte, ageIdentityPath string) ([]byte, error) {
	keyContent, err := os.ReadFile(ageIdentityPath)
	if err != nil {
		ci.logger.Error("failed to read age identity file", "path", ageIdentityPath, "error", err)
		return nil, fmt.Errorf("failed to read age identity file '%s': %w", ageIdentityPath, err)
	}

	identities, err := age.ParseIdentities(bytes.NewReader(keyContent))
	if err != nil {
		ci.logger.Error("failed to parse age identities", "path", ageIdentityPath, "error", err)
		return nil, fmt.Errorf("failed to parse age identities from '%s': %w", ageIdentityPath, err)
	}
	if len(identities) == 0 {
		return nil, fmt.Errorf("no age identities found in file '%s'", ageIdentityPath)
	}

	decryptReader, err := age.Decrypt(bytes.NewReader(encryptedData), identities...)
	if err != nil {
		ci.logger.Error("failed to create age decryption reader", "error", err)
		return nil, fmt.Errorf("failed to create age decryption reader: %w", err)
	}

	decryptedData, err := io.ReadAll(decryptReader)
	if err != nil {
		ci.logger.Error("failed to read decrypted data", "error", err)
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}
	return decryptedData, nil
}

// parseIdentityAndGetRecipient parses the age identity file and returns the first X25519 recipient.
func (ci *CertInserter) parseIdentityAndGetRecipient(ageIdentityPath string) (age.Recipient, error) {
	ageIdentityData, err := os.ReadFile(ageIdentityPath)
	if err != nil {
		ci.logger.Error("failed to read age identity file", "path", ageIdentityPath, "error", err)
		return nil, fmt.Errorf("failed to read age identity file '%s': %w", ageIdentityPath, err)
	}

	identities, err := age.ParseIdentities(bytes.NewReader(ageIdentityData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse age identity file '%s': %w", ageIdentityPath, err)
	}
	if len(identities) == 0 {
		return nil, fmt.Errorf("no age identities found in file '%s'", ageIdentityPath)
	}

	// Find the first X25519 identity to get its recipient (public key)
	for _, id := range identities {
		if x25519ID, ok := id.(*age.X25519Identity); ok {
			return x25519ID.Recipient(), nil
		}
	}

	ci.logger.Error("no X25519 age identity found in file - needed for encryption", "path", ageIdentityPath)
	return nil, fmt.Errorf("no X25519 age identity found in file '%s'", ageIdentityPath)
}

// encryptData encrypts data using the provided age recipient (public key).
func (ci *CertInserter) encryptData(data []byte, recipient age.Recipient) ([]byte, error) {
	encryptedOutput := &bytes.Buffer{}
	encryptWriter, err := age.Encrypt(encryptedOutput, recipient)
	if err != nil {
		ci.logger.Error("failed to create age encryption writer", "error", err)
		return nil, fmt.Errorf("failed to create age encryption writer: %w", err)
	}
	if _, err := io.Copy(encryptWriter, bytes.NewReader(data)); err != nil {
		ci.logger.Error("failed to write data to age encryption writer", "error", err)
		return nil, fmt.Errorf("failed to write data to age encryption writer: %w", err)
	}
	if err := encryptWriter.Close(); err != nil {
		ci.logger.Error("failed to close age encryption writer", "error", err)
		return nil, fmt.Errorf("failed to close age encryption writer: %w", err)
	}
	return encryptedOutput.Bytes(), nil
}

// insertEncryptedConfig inserts the encrypted config blob into the database.
func (ci *CertInserter) insertEncryptedConfig(ctx context.Context, encryptedData []byte, description string) error {
	conn, err := ci.pool.Take(ctx)
	if err != nil {
		ci.logger.Error("failed to get database connection for insert", "error", err)
		return fmt.Errorf("failed to get database connection for insert: %w", err)
	}
	defer ci.pool.Put(conn)

	now := time.Now().UTC().Format(time.RFC3339)

	err = sqlitex.Execute(conn,
		`INSERT INTO app_config (content, format, description, created_at) VALUES (?, ?, ?, ?)`,
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
	return nil
}

// UpdateConfigWithCert updates the configuration in the database with new certificate data.
func (ci *CertInserter) UpdateConfigWithCert(ctx context.Context, keyPath, certPath, ageIdentityPath string) error {
	// 1. Get current encrypted config
	encryptedConfig, err := ci.getLatestEncryptedConfig(ctx)
	if err != nil {
		return err // Error already logged and wrapped
	}

	// 2. Decrypt config
	decryptedToml, err := ci.decryptData(encryptedConfig, ageIdentityPath)
	if err != nil {
		return err // Error already logged and wrapped
	}

	// 3. Unmarshal config
	var cfg config.Config
	if err := toml.Unmarshal(decryptedToml, &cfg); err != nil {
		ci.logger.Error("failed to unmarshal TOML config", "error", err)
		return fmt.Errorf("failed to unmarshal TOML config: %w", err)
	}

	// 4. Read cert/key files
	keyDataBytes, err := os.ReadFile(keyPath)
	if err != nil {
		ci.logger.Error("failed to read key file", "path", keyPath, "error", err)
		return fmt.Errorf("failed to read key file '%s': %w", keyPath, err)
	}
	certDataBytes, err := os.ReadFile(certPath)
	if err != nil {
		ci.logger.Error("failed to read cert file", "path", certPath, "error", err)
		return fmt.Errorf("failed to read cert file '%s': %w", certPath, err)
	}

	// 5. Update config struct
	cfg.Server.KeyData = string(keyDataBytes)
	cfg.Server.CertData = string(certDataBytes)
	// Optionally clear file paths if data is now embedded
	// cfg.Server.KeyFile = ""
	// cfg.Server.CertFile = ""

	// 6. Marshal updated config back to TOML
	updatedTomlBytes, err := toml.Marshal(cfg)
	if err != nil {
		ci.logger.Error("failed to marshal updated config to TOML", "error", err)
		return fmt.Errorf("failed to marshal updated config to TOML: %w", err)
	}

	// 7. Get recipient (public key) for encryption
	recipient, err := ci.parseIdentityAndGetRecipient(ageIdentityPath)
	if err != nil {
		return err // Error already logged and wrapped
	}

	// 8. Encrypt updated config
	newEncryptedConfig, err := ci.encryptData(updatedTomlBytes, recipient)
	if err != nil {
		return err // Error already logged and wrapped
	}

	// 9. Insert new encrypted config version
	description := fmt.Sprintf("Updated TLS cert/key data from files: %s, %s", filepath.Base(keyPath), filepath.Base(certPath))
	if err := ci.insertEncryptedConfig(ctx, newEncryptedConfig, description); err != nil {
		return err // Error already logged and wrapped
	}

	ci.logger.Info("Successfully updated config with new TLS certificate data", "key_file", keyPath, "cert_file", certPath)
	return nil
}

func main() {
	keyPathFlag := flag.String("key", "", "Path to the TLS private key file (required)")
	certPathFlag := flag.String("cert", "", "Path to the TLS certificate file (required)")
	dbPathFlag := flag.String("db", "", "Path to the SQLite database file (required)")
	ageIdentityPathFlag := flag.String("age-key", "", "Path to the age identity file (containing private key 'AGE-SECRET-KEY-1...') (required)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -key <key-file> -cert <cert-file> -db <db-file> -age-key <identity-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Updates the application configuration in the database with TLS certificate data from files.\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *keyPathFlag == "" || *certPathFlag == "" || *dbPathFlag == "" || *ageIdentityPathFlag == "" {
		flag.Usage()
		os.Exit(1)
	}

	inserter := NewCertInserter(*dbPathFlag)
	if err := inserter.OpenDatabase(); err != nil {
		// Error already logged by OpenDatabase
		os.Exit(1)
	}
	defer inserter.pool.Close()

	ctx := context.Background() // Or use a context with timeout/cancellation
	if err := inserter.UpdateConfigWithCert(ctx, *keyPathFlag, *certPathFlag, *ageIdentityPathFlag); err != nil {
		// Error should be logged by UpdateConfigWithCert or its helpers
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) // Also print to stderr for visibility
		os.Exit(1)
	}
}
