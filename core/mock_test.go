package core

import (
	"context"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/notify"
	"github.com/caasmo/restinpieces/router"
)

// mockDbApp is a mock for db.DbApp
type mockDbApp struct {
	db.DbApp
}

// mockCache is a mock for cache.Cache
type mockCache struct{}

func (m *mockCache) Set(key string, value interface{}, cost int64) bool { return true }
func (m *mockCache) Get(key string) (interface{}, bool) {
	return nil, false
}
func (m *mockCache) Del(key string)                                          {}
func (m *mockCache) SetWithTTL(key string, value interface{}, cost int64, ttl time.Duration) bool {
	return true
}

// mockConfigStore is a mock for config.SecureStore
type mockConfigStore struct{}

func (m *mockConfigStore) Get(scope string, generation int) ([]byte, string, error) {
	return nil, "", nil
}
func (m *mockConfigStore) Save(scope string, plaintextData []byte, format string, description string) error {
	return nil
}

// mockNotifier is a mock for notify.Notifier
type mockNotifier struct{}

func (m *mockNotifier) Send(ctx context.Context, n notify.Notification) error {
	return nil
}

// mockConfigProvider is a manual mock for config.Provider
type mockConfigProvider struct {
	config.Provider
	getFunc func() *config.Config
}

func (m *mockConfigProvider) Get() *config.Config {
	if m.getFunc != nil {
		return m.getFunc()
	}
	return nil
}

// MockAuthenticator implements the Authenticator interface for testing
type MockAuth struct {
	// AuthenticateFunc allows customizing the authentication behavior
	AuthenticateFunc func(r *http.Request) (*db.User, jsonResponse, error)
}

func (m *MockAuth) Authenticate(r *http.Request) (*db.User, jsonResponse, error) {
	if m.AuthenticateFunc != nil {
		return m.AuthenticateFunc(r)
	}
	// Default: Return nil user with no error (unauthenticated)
	return nil, jsonResponse{}, nil
}

// MockValidator implements the Validator interface for testing
type MockValidator struct {
	ContentTypeFunc func(r *http.Request, allowedType string) (jsonResponse, error)
}

func (m *MockValidator) ContentType(r *http.Request, allowedType string) (jsonResponse, error) {
	return m.ContentTypeFunc(r, allowedType)
}

// MockRouter implements router.Router interface for testing
type MockRouter struct{}

func (m *MockRouter) Handle(path string, handler http.Handler)                                 {}
func (m *MockRouter) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) {}
func (m *MockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request)                         {}
func (m *MockRouter) Param(req *http.Request, key string) string                               { return "" }
func (m *MockRouter) Register(chains router.Chains)                                            {}