package prerouter

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

// mockNextHandler is a simple http.Handler that records if it was called.
type mockNextHandler struct {
	called bool
}

func (m *mockNextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.called = true
	w.WriteHeader(http.StatusOK)
}

func TestBlockUaList(t *testing.T) {
	testCases := []struct {
		name               string
		config             config.BlockUaList
		requestUserAgent   string
		expectedStatusCode int
		expectNextCalled   bool
	}{
		{
			name: "Case: Middleware is Inactive",
			config: config.BlockUaList{
				Activated: false,
				List:      config.Regexp{Regexp: regexp.MustCompile(`^BadBot/.*$`)},
			},
			requestUserAgent:   "BadBot/1.0",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: Matching User-Agent is Blocked",
			config: config.BlockUaList{
				Activated: true,
				List:      config.Regexp{Regexp: regexp.MustCompile(`^BadBot/.*$`)},
			},
			requestUserAgent:   "BadBot/1.0",
			expectedStatusCode: http.StatusForbidden,
			expectNextCalled:   false,
		},
		{
			name: "Case: Non-Matching User-Agent is Allowed",
			config: config.BlockUaList{
				Activated: true,
				List:      config.Regexp{Regexp: regexp.MustCompile(`^BadBot/.*$`)},
			},
			requestUserAgent:   "GoodBot/1.0",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: Request Has No User-Agent Header",
			config: config.BlockUaList{
				Activated: true,
				List:      config.Regexp{Regexp: regexp.MustCompile(`^BadBot/.*$`)},
			},
			requestUserAgent:   "", // No User-Agent header will be set
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
		{
			name: "Case: Middleware Active but Regex is Nil",
			config: config.BlockUaList{
				Activated: true,
				List:      config.Regexp{Regexp: nil}, // Simulate invalid or missing regex
			},
			requestUserAgent:   "AnyBot/1.0",
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup: Create a mock app and set the configuration for this test case.
			mockApp := &core.App{}
			cfg := &config.Config{
				BlockUaList: tc.config,
			}
			provider := config.NewProvider(cfg)
			mockApp.SetConfigProvider(provider)

			// Setup: Create the middleware instance.
			middleware := NewBlockUaList(mockApp)

			// Setup: Create a test request.
			req := httptest.NewRequest("GET", "/", nil)
			if tc.requestUserAgent != "" {
				req.Header.Set("User-Agent", tc.requestUserAgent)
			}

			// Setup: Create a response recorder and the mock next handler.
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
