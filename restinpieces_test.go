package restinpieces

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/db/mock"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/migrations"
	"github.com/pelletier/go-toml/v2"
	"zombiezen.com/go/sqlite/sqlitex"
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
	return config.NewDefaultConfig()
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

		if _, ok := app.Cache().(*ristretto.Cache[any]); !ok {
			t.Errorf("Expected cache of type *ristretto.Cache[any], but got %T", app.Cache())
		}
	})

	t.Run("Failure on invalid cache level", func(t *testing.T) {
		app := &core.App{}
		app.SetLogger(newTestLogger())
		init := &initializer{app: app}
		cfg := newTestConfig()
		cfg.Cache.Level = "invalid-level" // This should cause ristretto.New to fail

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

// TestNew_Unit validates the New function's initialization logic in a controlled environment.
func TestNew_Unit(t *testing.T) {
	// 1. Setup a Test Environment
	tempDir := t.TempDir()
	appDbPath := filepath.Join(tempDir, "app.db")
	logDbPath := filepath.Join(tempDir, "logs.db")
	_, ageKeyPath := newTestAgeIdentity(t)

	// 2. Initialize App Database for Setup
	setupConn, err := zombiezen.NewConn(appDbPath)
	if err != nil {
		t.Fatalf("Failed to create setup db connection: %v", err)
	}
	defer setupConn.Close()

	if err := zombiezen.ApplyMigrations(setupConn, migrations.Schema()); err != nil {
		t.Fatalf("Failed to apply migrations to app db: %v", err)
	}

	// 3. Initialize Log Database
	logSql, err := migrations.Schema().Open("log/logs.sql")
	if err != nil {
		t.Fatalf("Failed to open log schema file: %v", err)
	}
	defer logSql.Close()
	logSqlBytes, err := io.ReadAll(logSql)
	if err != nil {
		t.Fatalf("Failed to read log schema file: %v", err)
	}
	logConn, err := zombiezen.NewConn(logDbPath)
	if err != nil {
		t.Fatalf("Failed to create log db connection: %v", err)
	}
	if err := sqlitex.ExecuteScript(logConn, string(logSqlBytes), nil); err != nil {
		t.Fatalf("Failed to apply migrations to log db: %v", err)
	}
	if err := logConn.Close(); err != nil {
		t.Fatalf("Failed to close log db connection: %v", err)
	}

	// 4. Seed Encrypted Config using a temporary pool
	tempPool, err := sqlitex.NewPool(appDbPath, sqlitex.PoolOptions{PoolSize: 1})
	if err != nil {
		t.Fatalf("Failed to create temp pool for seeding: %v", err)
	}
	dbCfg, err := zombiezen.New(tempPool)
	if err != nil {
		t.Fatalf("Failed to create db config for seeding: %v", err)
	}
	secureStore, err := config.NewSecureStoreAge(dbCfg, ageKeyPath)
	if err != nil {
		t.Fatalf("Failed to create secure store: %v", err)
	}
	cfg := config.NewDefaultConfig()
	cfg.Log.Batch.DbPath = logDbPath // Point to our temp log db
	tomlBytes, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := secureStore.Save(config.ScopeApplication, tomlBytes, "toml", "initial test config"); err != nil {
		t.Fatalf("Failed to save config to secure store: %v", err)
	}
	tempPool.Close() // Close the temporary pool after seeding

	// 5. Execute with a fresh, real pool
	realPool, err := NewZombiezenPool(appDbPath)
	if err != nil {
		t.Fatalf("Failed to create real pool for New(): %v", err)
	}
	defer realPool.Close()

	app, srv, err := New(
		WithZombiezenPool(realPool),
		WithAgeKeyPath(ageKeyPath),
		WithLogger(newTestLogger()), // Provide a custom logger to prevent daemon start
	)

	// 6. Assertions
	if err != nil {
		t.Fatalf("New() returned an unexpected error: %v", err)
	}
	if app == nil {
		t.Fatal("New() returned a nil app")
	}
	if srv == nil {
		t.Fatal("New() returned a nil server")
	}
}