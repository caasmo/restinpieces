package prerouter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

// bodyReadingHandler is a simple http.Handler that attempts to read the request body.
// This is necessary to trigger the behavior of http.MaxBytesReader.
func bodyReadingHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Attempt to read the entire body.
		_, err := io.ReadAll(r.Body)
		if err != nil {
			// The httptest.Recorder doesn't automatically set the 413 status
			// like a real http.Server does. We must check for the specific error
			// returned by http.MaxBytesReader and set the status code manually
			// to accurately simulate the real-world behavior.
			if err.Error() == "http: request body too large" {
				// This is the error string produced by MaxBytesReader
				w.WriteHeader(http.StatusRequestEntityTooLarge)
			} else {
				// Handle other potential read errors if necessary
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		// If the read is successful, we write a 200 OK.
		w.WriteHeader(http.StatusOK)
	})
}

func TestBlockOversizedRequest(t *testing.T) {
	testCases := []struct {
		name               string
		config             config.BlockOversizedRequest
		requestURL         string
		requestBodySize    int
		requestHeaders     http.Header
		isGetRequest       bool
		expectedStatusCode int
	}{
		{
			name: "Case: Middleware is Inactive",
			config: config.BlockOversizedRequest{
				Activated: false,
				BodyLimit: 100,
			},
			requestURL:         "/",
			requestBodySize:    200,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Body is Under the Limit",
			config: config.BlockOversizedRequest{
				Activated: true,
				BodyLimit: 100,
			},
			requestURL:         "/",
			requestBodySize:    50,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Body is Exactly at the Limit",
			config: config.BlockOversizedRequest{
				Activated: true,
				BodyLimit: 100,
			},
			requestURL:         "/",
			requestBodySize:    100,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Body Exceeds the Limit",
			config: config.BlockOversizedRequest{
				Activated: true,
				BodyLimit: 100,
			},
			requestURL:         "/",
			requestBodySize:    101,
			expectedStatusCode: http.StatusRequestEntityTooLarge,
		},
		{
			name: "Case: URL Path Exceeds the Limit",
			config: config.BlockOversizedRequest{
				Activated:    true,
				URLPathLimit: 5,
			},
			requestURL:         "/toolong",
			expectedStatusCode: http.StatusRequestURITooLong,
		},
		{
			name: "Case: Query String Exceeds the Limit",
			config: config.BlockOversizedRequest{
				Activated:        true,
				QueryStringLimit: 5,
			},
			requestURL:         "/?a=12345",
			expectedStatusCode: http.StatusRequestURITooLong,
		},
		{
			name: "Case: Header Count Exceeds the Limit",
			config: config.BlockOversizedRequest{
				Activated:        true,
				HeaderCountLimit: 2,
			},
			requestURL: "/",
			requestHeaders: http.Header{
				"X-Header-1": {"v1"},
				"X-Header-2": {"v2"},
				"X-Header-3": {"v3"},
			},
			expectedStatusCode: http.StatusRequestHeaderFieldsTooLarge,
		},
		{
			name: "Case: Limits Disabled (Zero Values)",
			config: config.BlockOversizedRequest{
				Activated:        true,
				URLPathLimit:     0,
				QueryStringLimit: 0,
				HeaderCountLimit: 0,
				BodyLimit:        0,
			},
			requestURL: "/verylongpath?verylongquery=true",
			requestHeaders: http.Header{
				"X-Long-Header": {"very long header value"},
			},
			requestBodySize:    1000,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Has No Body (GET request)",
			config: config.BlockOversizedRequest{
				Activated: true,
				BodyLimit: 100,
			},
			requestURL:         "/",
			isGetRequest:       true,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Path is Excluded from Limit",
			config: config.BlockOversizedRequest{
				Activated:     true,
				BodyLimit:     100,
				ExcludedPaths: []string{"/upload"},
			},
			requestURL:         "/upload",
			requestBodySize:    200,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Path is Not Excluded from Limit",
			config: config.BlockOversizedRequest{
				Activated:     true,
				BodyLimit:     100,
				ExcludedPaths: []string{"/upload"},
			},
			requestURL:         "/api/data",
			requestBodySize:    200,
			expectedStatusCode: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup: Create a mock app and set the configuration.
			mockApp := &core.App{}
			cfg := &config.Config{
				BlockOversizedRequest: tc.config,
			}
			provider := config.NewProvider(cfg)
			mockApp.SetConfigProvider(provider)

			// Setup: Create the middleware instance.
			middleware := NewBlockOversizedRequest(mockApp)

			// Setup: Create the request body.
			var reqBody io.Reader
			if tc.requestBodySize > 0 {
				reqBody = strings.NewReader(strings.Repeat("a", tc.requestBodySize))
			}

			// Setup: Create the test request.
			method := "POST"
			if tc.isGetRequest {
				method = "GET"
			}
			req := httptest.NewRequest(method, tc.requestURL, reqBody)

			// Setup: Add headers if provided.
			if tc.requestHeaders != nil {
				req.Header = tc.requestHeaders
			}

			// Setup: Create a response recorder.
			rr := httptest.NewRecorder()

			// Execution: Chain the middleware with the body-reading handler.
			handler := middleware.Execute(bodyReadingHandler())
			handler.ServeHTTP(rr, req)

			// Verification: Check the status code.
			if rr.Code != tc.expectedStatusCode {
				t.Errorf("Expected status code %d, but got %d", tc.expectedStatusCode, rr.Code)
			}
		})
	}
}
