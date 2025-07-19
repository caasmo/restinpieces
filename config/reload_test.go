package config

import (
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// nullLogger returns a slog.Logger that discards all output.
func nullLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// mockSecureStore is a mock implementation of the SecureStore interface for testing.
type mockSecureStore struct {
	data []byte
	err  error
}

func (m *mockSecureStore) Get(scope string, generation int) ([]byte, string, error) {
	if m.err != nil {
		return nil, "", m.err
	}
	return m.data, "toml", nil
}

func (m *mockSecureStore) Save(scope string, plaintextData []byte, format string, description string) error {
	return errors.New("not implemented")
}

func TestCheckChangedRestartFields(t *testing.T) {
	t.Parallel()

	baseCfg := *NewDefaultConfig()

	testCases := []struct {
		name         string
		modifier     func(cfg *Config)
		expectFields []string
	}{
		{
			name:         "No changes",
			modifier:     func(cfg *Config) {},
			expectFields: []string{}, // Changed from nil
		},
		{
			name: "Safe field change (Maintenance.Activated)",
			modifier: func(cfg *Config) {
				cfg.Maintenance.Activated = !cfg.Maintenance.Activated
			},
			expectFields: []string{}, // Changed from nil
		},
		{
			name: "Restart field change (Server.Addr)",
			modifier: func(cfg *Config) {
				cfg.Server.Addr = ":9999"
			},
			expectFields: []string{"Server.Addr"},
		},
		{
			name: "Multiple restart fields changed",
			modifier: func(cfg *Config) {
				cfg.Server.EnableTLS = !baseCfg.Server.EnableTLS
				cfg.Server.RedirectAddr = ":8081"
			},
			expectFields: []string{"Server.EnableTLS", "Server.RedirectAddr"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			newCfg := baseCfg
			tc.modifier(&newCfg)

			changed := checkChangedRestartFields(&baseCfg, &newCfg)

			if !reflect.DeepEqual(changed, tc.expectFields) {
				t.Errorf("checkChangedRestartFields() got = %v, want %v", changed, tc.expectFields)
			}
		})
	}
}

func TestReload(t *testing.T) {
	t.Parallel()

	// Base config for tests
	oldCfg := NewDefaultConfig()

	t.Run("Success with no restart needed", func(t *testing.T) {
		t.Parallel()
		provider := NewProvider(oldCfg)

		newCfg := *NewDefaultConfig()
		newCfg.Maintenance.Activated = !oldCfg.Maintenance.Activated
		newCfgBytes, err := toml.Marshal(newCfg)
		if err != nil {
			t.Fatalf("Failed to marshal new config: %v", err)
		}

		mockStore := &mockSecureStore{data: newCfgBytes}
		reloadFn := Reload(mockStore, provider, nullLogger())
		err = reloadFn()

		if err != nil {
			t.Fatalf("reloadFn() returned unexpected error: %v", err)
		}

		updatedCfg := provider.Get()
		if updatedCfg.Maintenance.Activated == oldCfg.Maintenance.Activated {
			t.Error("Config was not updated after reload")
		}
	})

	t.Run("Success with restart needed", func(t *testing.T) {
		t.Parallel()
		provider := NewProvider(oldCfg)

		newCfg := *NewDefaultConfig()
		newCfg.Server.Addr = ":9999" // This field requires a restart
		newCfgBytes, err := toml.Marshal(newCfg)
		if err != nil {
			t.Fatalf("Failed to marshal new config: %v", err)
		}

		mockStore := &mockSecureStore{data: newCfgBytes}
		reloadFn := Reload(mockStore, provider, nullLogger())
		err = reloadFn()

		if err != nil {
			t.Fatalf("reloadFn() returned unexpected error: %v", err)
		}

		updatedCfg := provider.Get()
		if updatedCfg.Server.Addr != ":9999" {
			t.Error("Config was not updated, but it should have been")
		}
	})

	t.Run("SecureStore Get error", func(t *testing.T) {
		t.Parallel()
		provider := NewProvider(oldCfg)
		mockStore := &mockSecureStore{err: errors.New("db error")}

		reloadFn := Reload(mockStore, provider, nullLogger())
		err := reloadFn()

		if err == nil {
			t.Fatal("reloadFn() did not return an error when store failed")
		}
	})

	t.Run("Invalid TOML content", func(t *testing.T) {
		t.Parallel()
		provider := NewProvider(oldCfg)
		mockStore := &mockSecureStore{data: []byte("this is not valid toml")}

		reloadFn := Reload(mockStore, provider, nullLogger())
		err := reloadFn()

		if err == nil {
			t.Fatal("reloadFn() did not return an error for invalid TOML")
		}
	})

	t.Run("Validation error", func(t *testing.T) {
		t.Parallel()
		provider := NewProvider(oldCfg)
		// Create a valid config and then make it invalid
		newCfg := *NewDefaultConfig()
		newCfg.Jwt.AuthSecret = "" // This will fail validation
		newCfgBytes, err := toml.Marshal(newCfg)
		if err != nil {
			t.Fatalf("Failed to marshal new config: %v", err)
		}

		mockStore := &mockSecureStore{data: newCfgBytes}
		reloadFn := Reload(mockStore, provider, nullLogger())
		err = reloadFn()

		if err == nil {
			t.Fatal("reloadFn() did not return an error for a validation failure")
		}
	})
}
