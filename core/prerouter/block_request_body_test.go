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

func TestBlockRequestBody(t *testing.T) {
	testCases := []struct {
		name               string
		config             config.BlockRequestBody
		requestURL         string
		requestBodySize    int
		isGetRequest       bool
		expectedStatusCode int
	}{
		{
			name: "Case: Middleware is Inactive",
			config: config.BlockRequestBody{
				Activated: false,
				Limit:     100,
			},
			requestURL:         "/",
			requestBodySize:    200,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Body is Under the Limit",
			config: config.BlockRequestBody{
				Activated: true,
				Limit:     100,
			},
			requestURL:         "/",
			requestBodySize:    50,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Body is Exactly at the Limit",
			config: config.BlockRequestBody{
				Activated: true,
				Limit:     100,
			},
			requestURL:         "/",
			requestBodySize:    100,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Body Exceeds the Limit",
			config: config.BlockRequestBody{
				Activated: true,
				Limit:     100,
			},
			requestURL:         "/",
			requestBodySize:    101,
			expectedStatusCode: http.StatusRequestEntityTooLarge,
		},
		{
			name: "Case: Request Has No Body (GET request)",
			config: config.BlockRequestBody{
				Activated: true,
				Limit:     100,
			},
			requestURL:         "/",
			isGetRequest:       true,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Path is Excluded from Limit",
			config: config.BlockRequestBody{
				Activated:     true,
				Limit:         100,
				ExcludedPaths: []string{"/upload"},
			},
			requestURL:         "/upload",
			requestBodySize:    200,
			expectedStatusCode: http.StatusOK,
		},
		{
			name: "Case: Request Path is Not Excluded from Limit",
			config: config.BlockRequestBody{
				Activated:     true,
				Limit:         100,
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
				BlockRequestBody: tc.config,
			}
			provider := config.NewProvider(cfg)
			mockApp.SetConfigProvider(provider)

			// Setup: Create the middleware instance.
			middleware := NewBlockRequestBody(mockApp)

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
