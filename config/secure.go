package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath" // For Save description

	"filippo.io/age"
	"github.com/caasmo/restinpieces/db"
)

// ScopeApplication defines the scope for the main application configuration.
const ScopeApplication = "application"

// SecureStore defines an interface for securely storing and retrieving configuration data.
// Implementations handle the encryption/decryption details.
type SecureStore interface {
	// Latest retrieves the latest configuration for the given scope, decrypts it,
	// and returns the plaintext data.
	Latest(scope string) ([]byte, error)

	// Save encrypts the given plaintext data and stores it as the latest configuration
	// for the given scope, using the provided format and description.
	Save(scope string, plaintextData []byte, format string, description string) error
}

// secureStoreAge implements SecureStore using the age encryption library.
// It stores the path to the key file and re-parses identities on demand for decryption
// to minimize the time sensitive key material is held in memory.
type secureStoreAge struct {
	dbCfg      db.DbConfig
	ageKeyPath string // Path to the age private key file
}

// NewSecureStoreAge creates a new SecureStore implementation using age.
// It stores the necessary dependencies (db config, key path) for later use.
// Key file validation happens on the first call to Latest() or Save().
func NewSecureStoreAge(dbCfg db.DbConfig, ageKeyPath string) (SecureStore, error) {
	return &secureStoreAge{
		dbCfg:      dbCfg,
		ageKeyPath: ageKeyPath,
	}, nil
}

// loadAndParseIdentities reads the age key file, parses the identities,
// zeroes the raw key material, performs basic validation, and returns the identities.
// It's intended for internal use by Latest and Save.
func loadAndParseIdentities(keyPath string, operation string) ([]age.Identity, error) {
	keyContent, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("securestore: failed to read age key file '%s' for %s: %w", keyPath, operation, err)
	}

	identities, err := age.ParseIdentities(bytes.NewReader(keyContent))

	// Zero out the raw key material immediately after parsing attempt
	for i := range keyContent {
		keyContent[i] = 0
	}
	keyContent = nil // Help GC

	if err != nil {
		return nil, fmt.Errorf("securestore: failed to parse age identities from key file '%s' for %s: %w", keyPath, operation, err)
	}
	if len(identities) == 0 {
		return nil, fmt.Errorf("securestore: no age identities found in key file '%s' for %s", keyPath, operation)
	}

	// Ensure the first identity is the supported X25519 type
	if _, ok := identities[0].(*age.X25519Identity); !ok {
		err := fmt.Errorf("unsupported age identity type '%T' - must be X25519", identities[0])
		return nil, fmt.Errorf("securestore: %w", err)
	}

	return identities, nil
}

// Latest implements the SecureStore interface for age.
// It reads the key file and parses identities on demand for decryption.
func (s *secureStoreAge) Latest(scope string) ([]byte, error) {
	contentData, err := s.dbCfg.LatestConfig(scope)
	if err != nil {
		return nil, fmt.Errorf("securestore: failed to get latest config content for scope '%s' from db: %w", scope, err)
	}
	if len(contentData) == 0 {
		return nil, fmt.Errorf("securestore: no configuration content found for scope '%s'", scope)
	}

	identities, err := loadAndParseIdentities(s.ageKeyPath, "decryption")
	if err != nil {
		return nil, err // Return error directly
	}

	// Decrypt using the loaded identities
	contentDataReader := bytes.NewReader(contentData)
	decryptedDataReader, err := age.Decrypt(contentDataReader, identities...)

	if err != nil {
		return nil, fmt.Errorf("securestore: failed to decrypt configuration data for scope '%s': %w", scope, err)
	}

	// Read the decrypted result
	decryptedBytes, err := io.ReadAll(decryptedDataReader)
	if err != nil {
		return nil, fmt.Errorf("securestore: failed to read decrypted data stream for scope '%s': %w", scope, err)
	}

	return decryptedBytes, nil
}

// Save implements the SecureStore interface for age.
// It reads the key file, derives the recipient, and encrypts on demand.
func (s *secureStoreAge) Save(scope string, plaintextData []byte, format string, description string) error {
	identities, err := loadAndParseIdentities(s.ageKeyPath, "encryption")
	if err != nil {
		return err
	}

	// Derive recipient from the first loaded identity.
	recipient := identities[0].(*age.X25519Identity).Recipient()

	encryptedOutput := &bytes.Buffer{}
	encryptWriter, err := age.Encrypt(encryptedOutput, recipient)
	if err != nil {
		return fmt.Errorf("securestore: failed to create age encryption writer for scope '%s': %w", scope, err)
	}
	if _, err := io.Copy(encryptWriter, bytes.NewReader(plaintextData)); err != nil {
		return fmt.Errorf("securestore: failed to write data to age encryption writer for scope '%s': %w", scope, err)
	}
	if err := encryptWriter.Close(); err != nil {
		return fmt.Errorf("securestore: failed to close age encryption writer for scope '%s': %w", scope, err)
	}
	encryptedData := encryptedOutput.Bytes()

	// Insert encrypted data into DB
	err = s.dbCfg.InsertConfig(scope, encryptedData, format, description)
	if err != nil {
		return fmt.Errorf("securestore: failed to insert config for scope '%s': %w", scope, err)
	}

	return nil
}

func descriptionFromFile(filePath string) string {
	return "Inserted from file: " + filepath.Base(filePath)
}
