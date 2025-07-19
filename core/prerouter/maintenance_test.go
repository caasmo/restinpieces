package prerouter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

func TestMaintenanceMiddleware(t *testing.T) {
	testCases := []struct {
		name               string
		maintenanceActive  bool
		expectedStatusCode int
		expectNextCalled   bool
		expectedHeaders    map[string]string
	}{
		{
			name:               "Case: Maintenance Mode is Activated",
			maintenanceActive:  true,
			expectedStatusCode: http.StatusServiceUnavailable,
			expectNextCalled:   false,
			expectedHeaders:    core.HeadersMaintenancePage,
		},
		{
			name:               "Case: Maintenance Mode is Deactivated",
			maintenanceActive:  false,
			expectedStatusCode: http.StatusOK,
			expectNextCalled:   true,
			expectedHeaders:    nil, // No specific headers should be set by this middleware
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup: Create a mock app and set the configuration for this test case.
			mockApp := &core.App{}
			cfg := &config.Config{
				Maintenance: config.Maintenance{
					Activated: tc.maintenanceActive,
				},
			}
			provider := config.NewProvider(cfg)
			mockApp.SetConfigProvider(provider)

			// Setup: Create the middleware instance.
			middleware := NewMaintenance(mockApp)

			// Setup: Create a test request, response recorder, and mock next handler.
			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()
			next := &mockNextHandler{} // Using the same mock from other tests in this package

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

			// Verification: Check for expected headers.
			if tc.expectedHeaders != nil {
				for key, expectedValue := range tc.expectedHeaders {
					actualValue := rr.Header().Get(key)
					if actualValue != expectedValue {
						t.Errorf("Expected header '%s' to be '%s', but got '%s'", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}
