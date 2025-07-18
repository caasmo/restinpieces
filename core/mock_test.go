package core

import (
	"net/http"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
)

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
