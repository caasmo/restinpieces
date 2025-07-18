package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"github.com/caasmo/restinpieces/db/mock"
)

// newTestKey generates a new age key, writes it to a temporary file,
// and returns the path to the file.
func newTestKey(t *testing.T) string {
	t.Helper()
	key, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "key.txt")

	if err := os.WriteFile(keyPath, []byte(key.String()), 0600); err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	return keyPath
}

func TestSecureStore_SaveAndGet_Roundtrip(t *testing.T) {
	t.Parallel()

	keyPath := newTestKey(t)
	storage := make(map[string][]byte) // In-memory store for this test

	mockDB := &mock.Db{
		InsertConfigFunc: func(scope string, contentData []byte, format string, description string) error {
			storage[scope] = contentData
			return nil
		},
		GetConfigFunc: func(scope string, generation int) ([]byte, string, error) {
			data, ok := storage[scope]
			if !ok {
				return nil, "", errors.New("not found")
			}
			return data, "toml", nil
		},
	}

	store, err := NewSecureStoreAge(mockDB, keyPath)
	if err != nil {
		t.Fatalf("NewSecureStoreAge failed: %v", err)
	}

	want := []byte("my secret config")
	if err := store.Save(ScopeApplication, want, "toml", "test"); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	got, _, err := store.Get(ScopeApplication, 0)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("Get() got = %q, want %q", string(got), string(want))
	}
}

func TestGet_Failures(t *testing.T) {
	t.Parallel()

	keyPath := newTestKey(t)
	storage := make(map[string][]byte)

	// Setup: Encrypt data with the correct key and save it to our in-memory storage.
	// This is required for the "Decryption Failure" test case.
	{
		mockSetupDB := &mock.Db{
			InsertConfigFunc: func(scope string, contentData []byte, format string, description string) error {
				storage[scope] = contentData
				return nil
			},
		}
		setupStore, err := NewSecureStoreAge(mockSetupDB, keyPath)
		if err != nil {
			t.Fatalf("Setup store creation failed: %v", err)
		}
		if err := setupStore.Save(ScopeApplication, []byte("secret"), "toml", ""); err != nil {
			t.Fatalf("Setup save failed: %v", err)
		}
	}

	testCases := []struct {
		name      string
		store     SecureStore
		expectErr bool
	}{
		{
			name: "DB Error on Get",
			store: func() SecureStore {
				mockDB := &mock.Db{
					GetConfigFunc: func(scope string, generation int) ([]byte, string, error) {
						return nil, "", errors.New("db error")
					},
				}
				s, _ := NewSecureStoreAge(mockDB, keyPath)
				return s
			}(),
			expectErr: true,
		},
		{
			name: "Decryption Failure with wrong key",
			store: func() SecureStore {
				wrongKeyPath := newTestKey(t)
				mockDB := &mock.Db{
					GetConfigFunc: func(scope string, generation int) ([]byte, string, error) {
						return storage[scope], "toml", nil
					},
				}
				s, _ := NewSecureStoreAge(mockDB, wrongKeyPath)
				return s
			}(),
			expectErr: true,
		},
		{
			name: "Invalid Key File Path",
			store: func() SecureStore {
				s, _ := NewSecureStoreAge(&mock.Db{}, "/path/to/nonexistent/key.txt")
				return s
			}(),
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.store.Get(ScopeApplication, 0)
			if (err != nil) != tc.expectErr {
				t.Fatalf("Get() error = %v, expectErr %v", err, tc.expectErr)
			}
		})
	}
}

func TestLoadAndParseIdentities_Failures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		content string
	}{
		{"Malformed Key", "this-is-not-a-key"},
		{"Empty Key File", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			keyPath := filepath.Join(tempDir, "key.txt")
			if err := os.WriteFile(keyPath, []byte(tc.content), 0600); err != nil {
				t.Fatalf("Failed to write key file: %v", err)
			}

			_, err := loadAndParseIdentities(keyPath, "test")
			if err == nil {
				t.Error("loadAndParseIdentities() expected an error, but got nil")
			}
		})
	}
}