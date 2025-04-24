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
type secureConfigAge struct {
	dbCfg      db.DbConfig
	identities []age.Identity // For decryption
	recipient  age.Recipient  // For encryption
	logger     *slog.Logger
}

// NewSecureConfigAge creates a new SecureConfig implementation using age.
// It reads the age private key from ageKeyPath to initialize decryption identities
// and derives the encryption recipient (assuming X25519).
func NewSecureConfigAge(dbCfg db.DbConfig, ageKeyPath string, logger *slog.Logger) (SecureConfig, error) {
	keyContent, err := os.ReadFile(ageKeyPath)
	if err != nil {
		logger.Error("failed to read age key file", "path", ageKeyPath, "error", err)
		return nil, fmt.Errorf("secureconfig: failed to read age key file '%s': %w", ageKeyPath, err)
	}

	identities, err := age.ParseIdentities(bytes.NewReader(keyContent))
	// Zero out the raw key material immediately after parsing
	for i := range keyContent {
		keyContent[i] = 0
	}
	if err != nil {
		logger.Error("failed to parse age identities", "path", ageKeyPath, "error", err)
		return nil, fmt.Errorf("secureconfig: failed to parse age identities from key file '%s': %w", ageKeyPath, err)
	}
	if len(identities) == 0 {
		logger.Error("no age identities found in key file", "path", ageKeyPath)
		return nil, fmt.Errorf("secureconfig: no age identities found in key file '%s'", ageKeyPath)
	}

	// Derive recipient from the first identity (assuming X25519 for encryption)
	var recipient age.Recipient
	switch id := identities[0].(type) {
	case *age.X25519Identity:
		recipient = id.Recipient()
	default:
		logger.Error("unsupported age identity type for deriving recipient - must be X25519",
			"path", ageKeyPath,
			"type", fmt.Sprintf("%T", identities[0]))
		return nil, fmt.Errorf("secureconfig: unsupported age identity type '%T' for deriving recipient - must be X25519", identities[0])
	}

	return &secureConfigAge{
		dbCfg:      dbCfg,
		identities: identities,
		recipient:  recipient,
		logger:     logger.With("secure_config_type", "age"),
	}, nil
}

// Latest implements the SecureConfig interface for age.
func (s *secureConfigAge) Latest(scope string) ([]byte, error) {
	s.logger.Debug("fetching latest config content from db", "scope", scope)
	// 1. Get raw (encrypted) data from DB
	contentData, err := s.dbCfg.LatestConfig(scope)
	if err != nil {
		// Propagate DB error
		return nil, fmt.Errorf("secureconfig: failed to get latest config content for scope '%s' from db: %w", scope, err)
	}
	if len(contentData) == 0 {
		// Return specific error or nil? Let's return an error for clarity.
		s.logger.Warn("no configuration content found in database for scope", "scope", scope)
		return nil, fmt.Errorf("secureconfig: no configuration content found for scope '%s'", scope)
	}

	s.logger.Debug("decrypting config content", "scope", scope, "encrypted_size", len(contentData))
	// 2. Decrypt using stored identities
	encryptedDataReader := bytes.NewReader(contentData)
	decryptedDataReader, err := age.Decrypt(encryptedDataReader, s.identities...)
	if err != nil {
		s.logger.Error("failed to decrypt configuration data", "scope", scope, "error", err)
		return nil, fmt.Errorf("secureconfig: failed to decrypt configuration data for scope '%s': %w", scope, err)
	}

	decryptedBytes, err := io.ReadAll(decryptedDataReader)
	if err != nil {
		s.logger.Error("failed to read decrypted data stream", "scope", scope, "error", err)
		return nil, fmt.Errorf("secureconfig: failed to read decrypted data stream for scope '%s': %w", scope, err)
	}

	s.logger.Debug("successfully decrypted config content", "scope", scope, "decrypted_size", len(decryptedBytes))
	return decryptedBytes, nil
}

// Save implements the SecureConfig interface for age.
func (s *secureConfigAge) Save(scope string, plaintextData []byte, format string, description string) error {
	s.logger.Debug("encrypting config content for saving", "scope", scope, "plaintext_size", len(plaintextData))
	// 1. Encrypt using stored recipient
	encryptedOutput := &bytes.Buffer{}
	encryptWriter, err := age.Encrypt(encryptedOutput, s.recipient)
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

	// 2. Insert encrypted data into DB
	s.logger.Debug("inserting encrypted config content into db", "scope", scope, "format", format, "description", description)
	err = s.dbCfg.InsertConfig(scope, encryptedData, format, description)
	if err != nil {
		// dbCfg.InsertConfig should provide context, just wrap slightly
		s.logger.Error("failed to insert config content into db", "scope", scope, "error", err)
		return fmt.Errorf("secureconfig: failed to insert config for scope '%s': %w", scope, err)
	}

	s.logger.Info("successfully saved secure config", "scope", scope, "format", format)
	return nil
}

// Helper function (similar to insert-config) to create a default description
func descriptionFromFile(filePath string) string {
	return "Inserted from file: " + filepath.Base(filePath)
}
