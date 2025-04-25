package config

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath" // For Save description

	"filippo.io/age"
	"github.com/caasmo/restinpieces/db"
)

// SecureConfig defines an interface for securely storing and retrieving configuration data.
// Implementations handle the encryption/decryption details.
type SecureConfig interface {
	// Latest retrieves the latest configuration for the given scope, decrypts it,
	// and returns the plaintext data.
	Latest(scope string) ([]byte, error)

	// Save encrypts the given plaintext data and stores it as the latest configuration
	// for the given scope, using the provided format and description.
	Save(scope string, plaintextData []byte, format string, description string) error
}

// secureConfigAge implements SecureConfig using the age encryption library.
// It stores the path to the key file and re-parses identities on demand for decryption
// to minimize the time sensitive key material is held in memory.
type secureConfigAge struct {
	dbCfg      db.DbConfig
	ageKeyPath string // Path to the age private key file
	logger     *slog.Logger
}

// NewSecureConfigAge creates a new SecureConfig implementation using age.
// It stores the necessary dependencies (db config, key path, logger) for later use.
// Key file validation happens on the first call to Latest() or Save().
func NewSecureConfigAge(dbCfg db.DbConfig, ageKeyPath string, logger *slog.Logger) (SecureConfig, error) {
	return &secureConfigAge{
		dbCfg:      dbCfg,
		ageKeyPath: ageKeyPath,
		logger:     logger.With("secure_config_type", "age"),
	}, nil
}

// loadAndParseIdentities reads the age key file, parses the identities,
// zeroes the raw key material, performs basic validation, and returns the identities.
// It's intended for internal use by Latest and Save.
func loadAndParseIdentities(keyPath string, logger *slog.Logger, operation string) ([]age.Identity, error) {
	logger.Debug("reading age key file", "path", keyPath, "operation", operation)
	keyContent, err := os.ReadFile(keyPath)
	if err != nil {
		logger.Error("failed to read age key file", "path", keyPath, "operation", operation, "error", err)
		return nil, fmt.Errorf("secureconfig: failed to read age key file '%s' for %s: %w", keyPath, operation, err)
	}

	identities, err := age.ParseIdentities(bytes.NewReader(keyContent))

	// Zero out the raw key material immediately after parsing attempt
	for i := range keyContent {
		keyContent[i] = 0
	}
	keyContent = nil // Help GC

	if err != nil {
		logger.Error("failed to parse age identities", "path", keyPath, "operation", operation, "error", err)
		return nil, fmt.Errorf("secureconfig: failed to parse age identities from key file '%s' for %s: %w", keyPath, operation, err)
	}
	if len(identities) == 0 {
		logger.Error("no age identities found in key file", "path", keyPath, "operation", operation)
		return nil, fmt.Errorf("secureconfig: no age identities found in key file '%s' for %s", keyPath, operation)
	}

	// Ensure the first identity is the supported X25519 type
	if _, ok := identities[0].(*age.X25519Identity); !ok {
		err := fmt.Errorf("unsupported age identity type '%T' - must be X25519", identities[0])
		logger.Error("unsupported age identity type found", "path", keyPath, "operation", operation, "type", fmt.Sprintf("%T", identities[0]), "error", err)
		return nil, fmt.Errorf("secureconfig: %w", err)
	}

	return identities, nil
}

// Latest implements the SecureConfig interface for age.
// It reads the key file and parses identities on demand for decryption.
func (s *secureConfigAge) Latest(scope string) ([]byte, error) {
	s.logger.Debug("fetching latest config content from db", "scope", scope)
	contentData, err := s.dbCfg.LatestConfig(scope)
	if err != nil {
		return nil, fmt.Errorf("secureconfig: failed to get latest config content for scope '%s' from db: %w", scope, err)
	}
	if len(contentData) == 0 {
		s.logger.Warn("no configuration content found in database for scope", "scope", scope)
		return nil, fmt.Errorf("secureconfig: no configuration content found for scope '%s'", scope)
	}

	s.logger.Debug("decrypting config content", "scope", scope, "content_size", len(contentData))

	identities, err := loadAndParseIdentities(s.ageKeyPath, s.logger, "decryption")
	if err != nil {
		// Error already logged by helper function
		return nil, err // Return error directly
	}

	// Decrypt using the loaded identities
	contentDataReader := bytes.NewReader(contentData)
	decryptedDataReader, err := age.Decrypt(contentDataReader, identities...)

	if err != nil {
		s.logger.Error("failed to decrypt configuration data", "scope", scope, "error", err)
		return nil, fmt.Errorf("secureconfig: failed to decrypt configuration data for scope '%s': %w", scope, err)
	}

	// Read the decrypted result
	decryptedBytes, err := io.ReadAll(decryptedDataReader)
	if err != nil {
		s.logger.Error("failed to read decrypted data stream", "scope", scope, "error", err)
		return nil, fmt.Errorf("secureconfig: failed to read decrypted data stream for scope '%s': %w", scope, err)
	}

	s.logger.Debug("successfully decrypted config content", "scope", scope, "decrypted_size", len(decryptedBytes))
	return decryptedBytes, nil
}

// Save implements the SecureConfig interface for age.
// It reads the key file, derives the recipient, and encrypts on demand.
func (s *secureConfigAge) Save(scope string, plaintextData []byte, format string, description string) error {
	s.logger.Debug("encrypting config content for saving", "scope", scope, "plaintext_size", len(plaintextData))

	identities, err := loadAndParseIdentities(s.ageKeyPath, s.logger, "encryption")
	if err != nil {
		return err
	}

	// Derive recipient from the first loaded identity.
	recipient := identities[0].(*age.X25519Identity).Recipient()

	encryptedOutput := &bytes.Buffer{}
	encryptWriter, err := age.Encrypt(encryptedOutput, recipient)
	if err != nil {
		s.logger.Error("failed to create age encryption writer", "scope", scope, "error", err)
		return fmt.Errorf("secureconfig: failed to create age encryption writer for scope '%s': %w", scope, err)
	}
	if _, err := io.Copy(encryptWriter, bytes.NewReader(plaintextData)); err != nil {
		s.logger.Error("failed to write data to age encryption writer", "scope", scope, "error", err)
		return fmt.Errorf("secureconfig: failed to write data to age encryption writer for scope '%s': %w", scope, err)
	}
	if err := encryptWriter.Close(); err != nil {
		s.logger.Error("failed to close age encryption writer", "scope", scope, "error", err)
		return fmt.Errorf("secureconfig: failed to close age encryption writer for scope '%s': %w", scope, err)
	}
	encryptedData := encryptedOutput.Bytes()
	s.logger.Debug("successfully encrypted config content", "scope", scope, "encrypted_size", len(encryptedData))

	// Insert encrypted data into DB
	s.logger.Debug("inserting encrypted config content into db", "scope", scope, "format", format, "description", description)
	err = s.dbCfg.InsertConfig(scope, encryptedData, format, description)
	if err != nil {
		s.logger.Error("failed to insert config content into db", "scope", scope, "error", err)
		return fmt.Errorf("secureconfig: failed to insert config for scope '%s': %w", scope, err)
	}

	s.logger.Info("successfully saved secure config", "scope", scope, "format", format)
	return nil
}

func descriptionFromFile(filePath string) string {
	return "Inserted from file: " + filepath.Base(filePath)
}
