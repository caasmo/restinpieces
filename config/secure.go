package config

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"filippo.io/age"
	"github.com/caasmo/restinpieces/db"
)

// ScopeApplication defines the scope for the main application configuration.
const ScopeApplication = "application"

// SecureStore defines an interface for securely storing and retrieving configuration data.
// Implementations handle the encryption/decryption details.
type SecureStore interface {
	// Get retrieves configuration, decrypts it, and returns plaintext + format
	// empty scope = application scope
	// generation 0 = latest, 1 = previous, etc.
	Get(scope string, generation int) ([]byte, string, error)

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
// It validates the age key file exists and is readable immediately.
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
// TODO document return values
func (s *secureStoreAge) Get(scope string, generation int) ([]byte, string, error) {
	if generation < 0 {
		return nil, "", fmt.Errorf("generation cannot be negative")
	}

	if scope == "" {
		scope = ScopeApplication
	}

	encrypted, format, err := s.dbCfg.GetConfig(scope, generation)
	if err != nil {
		return nil, "", fmt.Errorf("securestore: failed to get config: %w", err)
	}

	identities, err := loadAndParseIdentities(s.ageKeyPath, "decryption")
	if err != nil {
		return nil, "", err
	}

	decrypted, err := age.Decrypt(bytes.NewReader(encrypted), identities...)
	if err != nil {
		return nil, "", fmt.Errorf("securestore: decrypt failed: %w", err)
	}

	plaintext, err := io.ReadAll(decrypted)
	return plaintext, format, err
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
