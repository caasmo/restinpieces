package prerouter

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caasmo/restinpieces/core"
)

func TestTLSHeaderSTS(t *testing.T) {
	// The expected value for the HSTS header, sourced directly from the core package.
	expectedHeaderValue := core.HeadersTls["Strict-Transport-Security"]

	testCases := []struct {
		name           string
		isTLS          bool // Controls whether the request simulates HTTPS
		expectHeader   bool // Controls whether we expect the HSTS header
	}{
		{
			name:         "Case: Request is over a TLS (HTTPS) Connection",
			isTLS:        true,
			expectHeader: true,
		},
		{
			name:         "Case: Request is over a non-TLS (HTTP) Connection",
			isTLS:        false,
			expectHeader: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup: Create the middleware instance.
			middleware := NewTLSHeaderSTS()

			// Setup: Create a test request.
			req := httptest.NewRequest("GET", "/", nil)
			if tc.isTLS {
				// To simulate an HTTPS request, we set a non-nil TLS field.
				req.TLS = &tls.ConnectionState{}
			}

			// Setup: Create a response recorder and a mock next handler.
			rr := httptest.NewRecorder()
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Execution: Create the handler chain and serve the request.
			handler := middleware.Execute(next)
			handler.ServeHTTP(rr, req)

			// Verification: Check the presence or absence of the header.
			headerValue := rr.Header().Get("Strict-Transport-Security")

			if tc.expectHeader {
				if headerValue == "" {
					t.Error("Expected 'Strict-Transport-Security' header to be set, but it was not")
				}
				if headerValue != expectedHeaderValue {
					t.Errorf("Expected header value '%s', but got '%s'", expectedHeaderValue, headerValue)
				}
			} else {
				if headerValue != "" {
					t.Errorf("Expected 'Strict-Transport-Security' header to be absent, but it was set to '%s'", headerValue)
				}
			}
		})
	}
}
