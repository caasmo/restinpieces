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
		// Document/HTML types (should use HeadersHtml)
		{
			name:            "root path",
			path:            "/",
			expectedHeaders: HeadersHtml,
		},
		{
			name:            "directory path with trailing slash",
			path:            "/query/",
			expectedHeaders: HeadersHtml,
		},
		{
			name:            "clean URL route without extension",
			path:            "/query",
			expectedHeaders: HeadersHtml,
		},
		{
			name:            "standard html file",
			path:            "/index.html",
			expectedHeaders: HeadersHtml,
		},
		{
			name:            "htm extension",
			path:            "/about.htm",
			expectedHeaders: HeadersHtml,
		},
		{
			name:            "nested clean URL",
			path:            "/settings/profile",
			expectedHeaders: HeadersHtml,
		},

		// Asset types (should use HeadersStatic)
		{
			name:            "css file",
			path:            "/assets/style.css",
			expectedHeaders: HeadersStatic,
		},
		{
			name:            "javascript file",
			path:            "/assets/app.js",
			expectedHeaders: HeadersStatic,
		},
		{
			name:            "png image",
			path:            "/images/logo.png",
			expectedHeaders: HeadersStatic,
		},
		{
			name:            "svg image",
			path:            "/images/icon.svg",
			expectedHeaders: HeadersStatic,
		},
		{
			name:            "json data file",
			path:            "/config.json",
			expectedHeaders: HeadersStatic,
		},
		{
			name:            "nested asset",
			path:            "/static/v1/theme/dark.css",
			expectedHeaders: HeadersStatic,
		},

		// Complex Paths (Query Strings, Fragments, etc.)
		{
			name:            "html with query string",
			path:            "/index.html?v=1.2.3",
			expectedHeaders: HeadersHtml,
		},
		{
			name:            "css with query string (cache busting)",
			path:            "/style.css?h=sha256-abc",
			expectedHeaders: HeadersStatic,
		},
		{
			name:            "clean URL with query string",
			path:            "/search?q=something",
			expectedHeaders: HeadersHtml,
		},
		{
			name:            "html with fragment (server only sees path)",
			path:            "/about.html",
			expectedHeaders: HeadersHtml,
		},
		{
			name:            "clean URL with fragment (server only sees path)",
			path:            "/docs",
			expectedHeaders: HeadersHtml,
		},
		{
			name:            "complex combined path (server only sees path and query)",
			path:            "/app/dashboard?user=123",
			expectedHeaders: HeadersHtml,
		},
		{
			name:            "asset with multiple dots",
			path:            "/assets/jquery.min.js",
			expectedHeaders: HeadersStatic,
		},
		{
			name:            "encoded characters in path",
			path:            "/my%20document.html",
			expectedHeaders: HeadersHtml,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()

			handler := StaticHeadersMiddleware(nextHandler)
			handler.ServeHTTP(rr, req)

			// Verify that the headers set by the middleware are correct.
			for key, expectedValue := range tc.expectedHeaders {
				got := rr.Header().Get(key)
				if got != expectedValue {
					t.Errorf("path %q: wrong value for header %q: got %q, want %q", tc.path, key, got, expectedValue)
				}
			}

			// Also verify we didn't get extra headers from the other set that shouldn't be there.
			// This ensures complete separation between document and asset headers.
			var otherHeaders map[string]string
			if tc.expectedHeaders["Cache-Control"] == HeadersHtml["Cache-Control"] {
				otherHeaders = HeadersStatic
			} else {
				otherHeaders = HeadersHtml
			}

			for key, unwantedValue := range otherHeaders {
				// Only check headers that are unique to the other set
				if _, ok := tc.expectedHeaders[key]; !ok {
					if got := rr.Header().Get(key); got == unwantedValue {
						t.Errorf("path %q: header %q should not have asset-specific value %q", tc.path, key, got)
					}
				}
			}
		})
	}
}
