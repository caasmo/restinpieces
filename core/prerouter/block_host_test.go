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
			name: "Case: Wildcard Domain Matches a Subdomain",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"*example.com"},
			},
			requestHost:        "api.example.com",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: Wildcard Domain Matches the Bare Domain",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"*example.com"},
			},
			requestHost:        "example.com",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
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
		{
			name: "Case: Wildcard Domain Does Not Match a Similar Domain",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"*example.com"},
			},
			requestHost:        "notexample.com",
			expectedStatusCode: http.StatusForbidden,
			expectNextCalled:   false,
		},
		{
			name: "Case: Exact Entry Does Not Match a Subdomain",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"example.com"},
			},
			requestHost:        "api.example.com",
			expectedStatusCode: http.StatusForbidden,
			expectNextCalled:   false,
		},
		{
			name: "Case: Host Case Differs from Allowed Entry",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"example.com"},
			},
			requestHost:        "Example.COM",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: Host Has a Trailing Dot",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"example.com"},
			},
			requestHost:        "example.com.",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: IPv6 Host with Port Matches Entry",
			config: config.BlockHost{
				Activated:    true,
				AllowedHosts: []string{"2001:db8::1"},
			},
			requestHost:        "[2001:db8::1]:8080",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
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
