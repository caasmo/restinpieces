package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStaticHeadersMiddleware(t *testing.T) {
	// A dummy handler that does nothing, to be wrapped by the middleware.
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	// Define test cases
	testCases := []struct {
		name            string
		path            string
		expectedHeaders map[string]string
	}{
		{
			name:            "request for html file",
			path:            "/index.html",
			expectedHeaders: headersStaticHtml,
		},
		{
			name:            "request for root path which should not be treated as html",
			path:            "/",
			expectedHeaders: headersStatic,
		},
		{
			name:            "request for css file",
			path:            "/assets/style.css",
			expectedHeaders: headersStatic,
		},
		{
			name:            "request for javascript file",
			path:            "/assets/app.js",
			expectedHeaders: headersStatic,
		},
		{
			name:            "request for image file",
			path:            "/images/logo.png",
			expectedHeaders: headersStatic,
		},
		{
			name:            "request with no file extension",
			path:            "/api/data",
			expectedHeaders: headersStatic,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()

			// Create the middleware handler and serve the request
			handler := StaticHeadersMiddleware(nextHandler)
			handler.ServeHTTP(rr, req)

			// Verify that the correct number of headers were set.
			if len(rr.Header()) != len(tc.expectedHeaders) {
				t.Errorf("handler returned wrong number of headers: got %d, want %d", len(rr.Header()), len(tc.expectedHeaders))
				t.Logf("Got headers: %v", rr.Header())
				t.Logf("Want headers: %v", tc.expectedHeaders)
			}

			// Verify that the headers set by the middleware are correct.
			for key, expectedValue := range tc.expectedHeaders {
				if got := rr.Header().Get(key); got != expectedValue {
					t.Errorf("handler returned wrong value for header %q: got %q, want %q", key, got, expectedValue)
				}
			}
		})
	}
}