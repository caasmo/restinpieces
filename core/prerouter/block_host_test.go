package prerouter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

func TestBlockHost(t *testing.T) {
	testCases := []struct {
		name               string
		config             config.BlockHost
		requestHost        string
		expectedStatusCode int
		expectNextCalled   bool
	}{
		{
			name: "Case: Middleware is Inactive",
			config: config.BlockHost{
				Activated:    false,
				AllowedHosts: []string{"example.com"},
			},
			requestHost:        "unauthorized.com",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: Host is in Allowed List (Exact Match)",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"example.com", "api.example.com"},
			},
			requestHost:        "example.com",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: Host is Not in Allowed List",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"example.com"},
			},
			requestHost:        "unauthorized.com",
			expectedStatusCode: http.StatusForbidden,
			expectNextCalled:   false,
		},
		{
			name: "Case: Host Matches a Wildcard Subdomain",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"*.example.com"},
			},
			requestHost:        "api.example.com",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: Host is Bare Domain and Wildcard Exists",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"*.example.com"},
			},
			requestHost:        "example.com",
			expectedStatusCode: http.StatusForbidden,
			expectNextCalled:   false,
		},
		{
			name: "Case: AllowedHosts List is Empty",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{},
			},
			requestHost:        "anyhost.com",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: Request Host Includes a Port (Allowed)",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"example.com"},
			},
			requestHost:        "example.com:8080",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: Request Host Includes a Port (Blocked)",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"example.com"},
			},
			requestHost:        "unauthorized.com:8080",
			expectedStatusCode: http.StatusForbidden,
			expectNextCalled:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup: Create a mock app and set the configuration.
			mockApp := &core.App{}
			cfg := &config.Config{
				BlockHost: tc.config,
			}
			provider := config.NewProvider(cfg)
			mockApp.SetConfigProvider(provider)

			// Setup: Create the middleware instance.
			middleware := NewBlockHost(mockApp)

			// Setup: Create a test request with the specified Host header.
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tc.requestHost

			// Setup: Create a response recorder and a mock next handler.
			rr := httptest.NewRecorder()
			next := &mockNextHandler{}

			// Execution: Create the handler chain and serve the request.
			handler := middleware.Execute(next)
			handler.ServeHTTP(rr, req)

			// Verification: Check the status code.
			if rr.Code != tc.expectedStatusCode {
				t.Errorf("Expected status code %d, but got %d", tc.expectedStatusCode, rr.Code)
			}

			// Verification: Check if the next handler was called.
			if next.called != tc.expectNextCalled {
				t.Errorf("Expected next handler called to be %v, but it was %v", tc.expectNextCalled, next.called)
			}
		})
	}
}
