package core

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

// TestAppInitialization verifies that a new App instance has nil fields.
func TestAppInitialization(t *testing.T) {
	app := &App{}

	if app.Router() != nil {
		t.Error("expected Router to be nil on initialization")
	}
	if app.DbAuth() != nil {
		t.Error("expected DbAuth to be nil on initialization")
	}
	if app.DbQueue() != nil {
		t.Error("expected DbQueue to be nil on initialization")
	}
	if app.Logger() != nil {
		t.Error("expected Logger to be nil on initialization")
	}
	if app.Cache() != nil {
		t.Error("expected Cache to be nil on initialization")
	}
	if app.ConfigStore() != nil {
		t.Error("expected ConfigStore to be nil on initialization")
	}
	if app.Notifier() != nil {
		t.Error("expected Notifier to be nil on initialization")
	}
	if app.Auth() != nil {
		t.Error("expected Authenticator to be nil on initialization")
	}
	if app.Validator() != nil {
		t.Error("expected Validator to be nil on initialization")
	}
	// configProvider is not checked because Config() would panic.
	// Its test is separate.
}

// TestAppSettersAndGetters ensures that each setter correctly assigns a component
// and the corresponding getter retrieves the exact same instance.
func TestAppSettersAndGetters(t *testing.T) {
	app := &App{}

	// Mock components
	mockRouter := &MockRouter{}
	mockCache := &mockCache{}
	mockConfigStore := &mockConfigStore{}
	mockLogger := slog.Default()
	mockNotifier := &mockNotifier{}
	mockAuthenticator := &MockAuth{}
	mockValidator := &MockValidator{}
	mockProvider := config.NewProvider(&config.Config{})

	testCases := []struct {
		name     string
		setter   func()
		getter   func() interface{}
		expected interface{}
	}{
		{
			name:     "Router",
			setter:   func() { app.SetRouter(mockRouter) },
			getter:   func() interface{} { return app.Router() },
			expected: mockRouter,
		},
		{
			name:     "Cache",
			setter:   func() { app.SetCache(mockCache) },
			getter:   func() interface{} { return app.Cache() },
			expected: mockCache,
		},
		{
			name:     "ConfigStore",
			setter:   func() { app.SetConfigStore(mockConfigStore) },
			getter:   func() interface{} { return app.ConfigStore() },
			expected: mockConfigStore,
		},
		{
			name:     "Logger",
			setter:   func() { app.SetLogger(mockLogger) },
			getter:   func() interface{} { return app.Logger() },
			expected: mockLogger,
		},
		{
			name:     "Notifier",
			setter:   func() { app.SetNotifier(mockNotifier) },
			getter:   func() interface{} { return app.Notifier() },
			expected: mockNotifier,
		},
		{
			name:     "Authenticator",
			setter:   func() { app.SetAuthenticator(mockAuthenticator) },
			getter:   func() interface{} { return app.Auth() },
			expected: mockAuthenticator,
		},
		{
			name:     "Validator",
			setter:   func() { app.SetValidator(mockValidator) },
			getter:   func() interface{} { return app.Validator() },
			expected: mockValidator,
		},
		{
			name:     "ConfigProvider",
			setter:   func() { app.SetConfigProvider(mockProvider) },
			getter:   func() interface{} { return app.configProvider },
			expected: mockProvider,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setter()
			result := tc.getter()
			if result != tc.expected {
				t.Errorf("getter for %s returned incorrect instance", tc.name)
			}
		})
	}
}

// TestApp_SetDb verifies the logic of the SetDb method.
func TestApp_SetDb(t *testing.T) {
	t.Run("Successful assignment", func(t *testing.T) {
		app := &App{}
		mockDb := &mockDbApp{}
		app.SetDb(mockDb)

		if app.DbAuth() != mockDb {
			t.Error("DbAuth was not set correctly")
		}
		if app.DbQueue() != mockDb {
			t.Error("DbQueue was not set correctly")
		}
	})

	t.Run("Panic on nil assignment", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("The code did not panic when setting a nil DbApp")
			}
		}()
		app := &App{}
		app.SetDb(nil)
	})
}

// TestApp_Config verifies that the Config method correctly uses the config provider.
func TestApp_Config(t *testing.T) {
	app := &App{}
	expectedConfig := &config.Config{}

	// Use a real provider instead of a mock
	provider := config.NewProvider(expectedConfig)
	app.SetConfigProvider(provider)

	resultConfig := app.Config()

	if !reflect.DeepEqual(resultConfig, expectedConfig) {
		t.Errorf("Config() returned %+v, want %+v", resultConfig, expectedConfig)
	}
}
