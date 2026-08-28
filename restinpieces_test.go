package restinpieces

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/db/mock"
	"github.com/pelletier/go-toml/v2"
)

// --- Test Helpers ---

// newTestAgeIdentity creates a new age identity and saves the private key to a
// temporary file. It returns the identity and the path to the key file.
func newTestAgeIdentity(t *testing.T) (*age.X25519Identity, string) {
	t.Helper()
	key, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate age key: %v", err)
	}

	keyFile := filepath.Join(t.TempDir(), "age.key")
	if err := os.WriteFile(keyFile, []byte(key.String()), 0600); err != nil {
		t.Fatalf("Failed to write age key to file: %v", err)
	}

	return key, keyFile
}

// newTestConfig creates a default configuration for testing purposes.
func newTestConfig() *config.Config {
	cfg := config.NewDefaultConfig()
	cfg.Jwt.AuthSecret = "test_auth_secret_32_chars_long__"
	cfg.Jwt.PasswordResetSecret = "test_pwreset_secret_32_chars___"
	cfg.Jwt.EmailChangeOtpSecret = "test_ec_otp_secret_32_chars____"
	cfg.Jwt.VerificationEmailOtpSecret = "test_ve_otp_secret_32_chars____"
	cfg.Jwt.Oauth2StateSecret = "test_oauth2_state_secret_32_ch_"
	return cfg
}

// newTestLogger creates a silent logger for tests to avoid noisy output.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- Unit Tests for Helper Methods ---

func TestGetLogDbPath(t *testing.T) {
	testCases := []struct {
		name         string
		cfgLogDbPath string
		mainDbPath   string
		expectedPath string
		expectErr    bool
	}{
		{
			name:         "Path from config",
			cfgLogDbPath: "/custom/path/logs.db",
			mainDbPath:   "/data/main.db", // This will be ignored
			expectedPath: "/custom/path/logs.db",
			expectErr:    false,
		},
		{
			name:         "Path derived from main db path",
			cfgLogDbPath: "",
			mainDbPath:   "/data/main.db",
			expectedPath: "/data/logs.db",
			expectErr:    false,
		},
		{
			name:         "Path derived from main db path in same directory",
			cfgLogDbPath: "",
			mainDbPath:   "main.db",
			expectedPath: "logs.db",
			expectErr:    false,
		},
		{
			name:         "Error when main db path is missing",
			mainDbPath:   "",
			expectErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Log: config.Log{
					Batch: config.BatchLogger{
						DbPath: tc.cfgLogDbPath,
					},
				},
			}
			// Use the mock that correctly implements the interface
			dbCfg := &mock.Db{}
			// Control the mock's Path() method for predictable results
			dbCfg.PathFunc = func() string {
				return tc.mainDbPath
			}

			path, err := getLogDbPath(cfg, dbCfg)

			if tc.expectErr {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("Did not expect an error but got: %v", err)
				}
				if path != tc.expectedPath {
					t.Errorf("Expected path '%s', got '%s'", tc.expectedPath, path)
				}
			}
		})
	}
}

func TestSetupDefaultRouter(t *testing.T) {
	app := &core.App{}
	app.SetLogger(newTestLogger())

	init := &initializer{app: app}
	err := init.setupDefaultRouter()
	if err != nil {
		t.Fatalf("setupDefaultRouter() returned an unexpected error: %v", err)
	}

	if app.Router() == nil {
		t.Fatal("app.Router() is nil after calling setupDefaultRouter()")
	}

	// Check if the router implements http.Handler, which is a safe, non-brittle check.
	var _ http.Handler = app.Router()
}

func TestSetupDefaultCache(t *testing.T) {
	t.Run("Successful cache creation", func(t *testing.T) {
		app := &core.App{}
		app.SetLogger(newTestLogger())
		init := &initializer{app: app}
		cfg := newTestConfig()

		err := init.setupDefaultCache(cfg)
		if err != nil {
			t.Fatalf("setupDefaultCache() returned an unexpected error: %v", err)
		}

		if app.Cache() == nil {
			t.Fatal("app.Cache() is nil after calling setupDefaultCache()")
		}
	})

	t.Run("Failure on invalid cache level", func(t *testing.T) {
		app := &core.App{}
		app.SetLogger(newTestLogger())
		init := &initializer{app: app}
		cfg := newTestConfig()
		cfg.Cache.Level = "invalid-level" // This should cause cache.New to fail

		err := init.setupDefaultCache(cfg)
		if err == nil {
			t.Fatal("setupDefaultCache() did not return an error on invalid cache level")
		}
	})
}

func TestSetupConfig(t *testing.T) {
	identity, ageKeyPath := newTestAgeIdentity(t)

	t.Run("Successful config loading", func(t *testing.T) {
		// 1. Prepare a valid config and encrypt it.
		cfg := newTestConfig()
		tomlBytes, err := toml.Marshal(cfg)
		if err != nil {
			t.Fatalf("Failed to marshal config: %v", err)
		}

		encryptedBytes := &bytes.Buffer{}
		w, err := age.Encrypt(encryptedBytes, identity.Recipient())
		if err != nil {
			t.Fatalf("Failed to create encryptor: %v", err)
		}
		if _, err := w.Write(tomlBytes); err != nil {
			t.Fatalf("Failed to write encrypted data: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Failed to close encryptor: %v", err)
		}

		// 2. Configure the mock DB to return the encrypted data.
		dbCfg := &mock.Db{}
		dbCfg.GetConfigFunc = func(scope string, generation int) ([]byte, string, error) {
			return encryptedBytes.Bytes(), "toml", nil
		}

		// 3. Setup the initializer and call the function.
		app := &core.App{}
		init := &initializer{
			app:        app,
			dbConfig:   dbCfg,
			ageKeyPath: ageKeyPath,
		}

		provider, err := init.setupConfig()

		// 4. Assertions.
		if err != nil {
			t.Fatalf("setupConfig() returned an unexpected error: %v", err)
		}
		if provider == nil {
			t.Fatal("setupConfig() returned a nil provider")
		}
		if app.Config() == nil {
			t.Fatal("app.Config() is nil after calling setupConfig()")
		}
		loadedCfg := app.Config()
		if loadedCfg.Server.Addr != cfg.Server.Addr {
			t.Errorf("Loaded config does not match original. Got %s, want %s", loadedCfg.Server.Addr, cfg.Server.Addr)
		}
	})

	t.Run("Failure on db GetConfig error", func(t *testing.T) {
		dbCfg := &mock.Db{}
		dbCfg.GetConfigFunc = func(scope string, generation int) ([]byte, string, error) {
			return nil, "", fmt.Errorf("forced db error")
		}
		app := &core.App{}
		init := &initializer{
			app:        app,
			dbConfig:   dbCfg,
			ageKeyPath: ageKeyPath,
		}
		_, err := init.setupConfig()
		if err == nil {
			t.Fatal("Expected an error but got none")
		}
	})

	t.Run("Failure on invalid TOML data", func(t *testing.T) {
		// Encrypt invalid data
		encryptedBytes := &bytes.Buffer{}
		w, _ := age.Encrypt(encryptedBytes, identity.Recipient())
		_, _ = w.Write([]byte("this is not valid toml"))
		_ = w.Close()

		dbCfg := &mock.Db{}
		dbCfg.GetConfigFunc = func(scope string, generation int) ([]byte, string, error) {
			return encryptedBytes.Bytes(), "toml", nil
		}
		app := &core.App{}
		init := &initializer{
			app:        app,
			dbConfig:   dbCfg,
			ageKeyPath: ageKeyPath,
		}
		_, err := init.setupConfig()
		if err == nil {
			t.Fatal("Expected an error but got none")
		}
	})

	t.Run("Failure on invalid config validation", func(t *testing.T) {
		// Encrypt a config that will fail validation
		cfg := newTestConfig()
		cfg.Server.Addr = "" // Invalid
		tomlBytes, _ := toml.Marshal(cfg)
		encryptedBytes := &bytes.Buffer{}
		w, _ := age.Encrypt(encryptedBytes, identity.Recipient())
		_, _ = w.Write(tomlBytes)
		_ = w.Close()

		dbCfg := &mock.Db{}
		dbCfg.GetConfigFunc = func(scope string, generation int) ([]byte, string, error) {
			return encryptedBytes.Bytes(), "toml", nil
		}
		app := &core.App{}
		init := &initializer{
			app:        app,
			dbConfig:   dbCfg,
			ageKeyPath: ageKeyPath,
		}
		_, err := init.setupConfig()
		if err == nil {
			t.Fatal("Expected an error but got none")
		}
	})
}

// TestNew_MissingDB validates that New returns an error when the database is not provided.
func TestNew_MissingDB(t *testing.T) {
	_, ageKeyPath := newTestAgeIdentity(t)

	// Attempt to create a new app without providing the database
	_, _, err := New(
		WithAgeKeyPath(ageKeyPath),
		WithLogger(newTestLogger()),
	)

	// Assert that an error is returned
	if err == nil {
		t.Fatal("New() did not return an error when the database was not provided")
	}

	// Optional: Check for a specific error message to make the test more robust
	expectedError := "DbAuth is required but was not provided (use WithDbApp)"
	if err.Error() != expectedError {
		t.Errorf("New() returned an unexpected error. Got: %v, Want: %v", err, expectedError)
	}
}

func TestNew_WithUserLogger(t *testing.T) {
	identity, ageKeyPath := newTestAgeIdentity(t)

	cfg := newTestConfig()
	tomlBytes, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	encryptedBytes := &bytes.Buffer{}
	w, err := age.Encrypt(encryptedBytes, identity.Recipient())
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}
	_, err = w.Write(tomlBytes)
	if err != nil {
		t.Fatalf("Failed to write encrypted data: %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("Failed to close encryptor: %v", err)
	}

	dbMock := &mock.Db{}
	dbMock.GetConfigFunc = func(scope string, generation int) ([]byte, string, error) {
		return encryptedBytes.Bytes(), "toml", nil
	}
	dbMock.PathFunc = func() string {
		return filepath.Join(t.TempDir(), "app.db")
	}

	customLogger := newTestLogger()
	app, srv, err := New(
		WithDbApp(dbMock),
		func(i *initializer) {
			i.dbConfig = dbMock
		},
		WithAgeKeyPath(ageKeyPath),
		WithLogger(customLogger),
	)
	if err != nil {
		t.Fatalf("New() returned unexpected error: %v", err)
	}
	if app == nil {
		t.Fatal("New() returned nil app")
	}
	if srv == nil {
		t.Fatal("New() returned nil server")
	}
	if app.Logger() != customLogger {
		t.Error("expected custom logger to be preserved")
	}
	if app.Config() == nil {
		t.Fatal("app.Config() is nil after New()")
	}
	if app.Router() == nil {
		t.Fatal("app.Router() is nil after New()")
	}
	if app.Cache() == nil {
		t.Fatal("app.Cache() is nil after New()")
	}
}

func TestSetupDefaultLogger_WithUserLogger(t *testing.T) {
	app := &core.App{}
	customLogger := newTestLogger()
	app.SetLogger(customLogger)

	init := &initializer{app: app}
	cfg := newTestConfig()
	provider := config.NewProvider(cfg)

	daemon, err := init.setupDefaultLogger(provider, true)
	if err != nil {
		t.Fatalf("setupDefaultLogger with user logger should not return error, got %v", err)
	}
	if daemon != nil {
		t.Error("expected nil daemon when user logger is provided")
	}
	if app.Logger() != customLogger {
		t.Error("expected custom logger to stay unchanged")
	}
}

func TestSetupDefaultLogger_GetLogDbPathError(t *testing.T) {
	app := &core.App{}
	app.SetLogger(newTestLogger())

	dbMock := &mock.Db{}
	dbMock.PathFunc = func() string {
		return ""
	}

	init := &initializer{
		app:      app,
		dbConfig: dbMock,
	}

	cfg := newTestConfig()
	cfg.Log.Batch.DbPath = ""
	provider := config.NewProvider(cfg)

	_, err := init.setupDefaultLogger(provider, false)
	if err == nil {
		t.Fatal("expected error when log db path cannot be determined")
	}
	if !strings.Contains(err.Error(), "cannot determine log database path") {
		t.Errorf("expected error about log database path, got %v", err)
	}
}

func TestSetupDefaultLogger_ConnNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "app.db")

	app := &core.App{}
	app.SetLogger(newTestLogger())

	dbMock := &mock.Db{}
	dbMock.PathFunc = func() string {
		return mainPath
	}

	init := &initializer{
		app:      app,
		dbConfig: dbMock,
	}

	cfg := newTestConfig()
	cfg.Log.Batch.DbPath = ""
	provider := config.NewProvider(cfg)

	_, err := init.setupDefaultLogger(provider, false)
	if err == nil {
		t.Fatal("expected error when log database file is missing")
	}
	if !strings.Contains(err.Error(), "log database not found") {
		t.Errorf("expected 'log database not found' error, got %v", err)
	}
}

func TestNewLog_FileNotFound(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "logs.db")

	init := &initializer{
		app: &core.App{},
	}

	_, err := init.newLog(logPath)
	if err == nil {
		t.Fatal("expected error when log database file is missing")
	}
}
