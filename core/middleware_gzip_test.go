package core

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// Helper to create a gzipped version of a string using only standard library.
func newGzippedBytes(t *testing.T, content string) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write to gzip writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}
	return b.Bytes()
}

// nonSeekableFile wraps an fs.File to make it non-seekable for testing purposes
// by only embedding the methods of fs.File, which does not include io.Seeker.
type nonSeekableFile struct {
	fs.File
}

// nonSeekableFS wraps an fs.FS to return non-seekable files.
type nonSeekableFS struct {
	fs.FS
}

// Open returns a non-seekable version of the requested file.
func (nfs *nonSeekableFS) Open(name string) (fs.File, error) {
	f, err := nfs.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return &nonSeekableFile{f}, nil
}

func TestGzipMiddleware(t *testing.T) {
	// --- Test Setup ---
	const (
		htmlContent = "<!DOCTYPE html><html><body><h1>Hello, Gzip!</h1></body></html>"
		cssContent  = "body { background-color: #f0f0f0; }"
	)

	// Create a mock filesystem with original and gzipped files
	mockFS := fstest.MapFS{
		"index.html":       {Data: []byte(htmlContent)},
		"index.html.gz":    {Data: newGzippedBytes(t, htmlContent)},
		"assets/style.css": {Data: []byte(cssContent)},
		// no gzipped version for style.css to test fallback
	}

	// A fallback handler that serves files from the mock filesystem
	fallbackHandler := http.FileServer(http.FS(mockFS))

	// --- Test Cases ---
	testCases := []struct {
		name                string
		path                string
		acceptEncoding      string
		method              string
		expectedStatus      int
		expectedContent     []byte
		expectedGzipped     bool
		expectedContentType string
	}{
		{
			name:                "serves gzipped file when available and accepted",
			path:                "/index.html",
			acceptEncoding:      "gzip, deflate, br",
			method:              http.MethodGet,
			expectedStatus:      http.StatusOK,
			expectedContent:     newGzippedBytes(t, htmlContent),
			expectedGzipped:     true,
			expectedContentType: "text/html; charset=utf-8",
		},
		{
			name:                "falls back to uncompressed when gzipped file is missing",
			path:                "/assets/style.css",
			acceptEncoding:      "gzip",
			method:              http.MethodGet,
			expectedStatus:      http.StatusOK,
			expectedContent:     []byte(cssContent),
			expectedGzipped:     false,
			expectedContentType: "text/css; charset=utf-8",
		},
		{
			name:                "falls back to uncompressed when client does not accept gzip",
			path:                "/", // Use root path, http.FileServer will serve index.html
			acceptEncoding:      "identity",
			method:              http.MethodGet,
			expectedStatus:      http.StatusOK,
			expectedContent:     []byte(htmlContent),
			expectedGzipped:     false,
			expectedContentType: "text/html; charset=utf-8",
		},
		{
			name:                "skips middleware for non-GET requests",
			path:                "/", // Use root path
			acceptEncoding:      "gzip",
			method:              http.MethodPost,
			expectedStatus:      http.StatusMethodNotAllowed,
			expectedContent:     []byte{},
			expectedGzipped:     false,
		},
		{
			name:                "handles request for non-existent file",
			path:                "/not-found.txt",
			acceptEncoding:      "gzip",
			method:              http.MethodGet,
			expectedStatus:      http.StatusNotFound,
			expectedContent:     []byte("404 page not found\n"),
			expectedContentType: "text/plain; charset=utf-8",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- Execution ---
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Accept-Encoding", tc.acceptEncoding)
			rr := httptest.NewRecorder()

			// A fallback handler that serves files from the mock filesystem
			var nextHandler http.Handler
			if tc.method == http.MethodPost {
				// For the POST test, use a specific handler that we know will return 405
				nextHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusMethodNotAllowed)
				})
			} else {
				nextHandler = fallbackHandler
			}

			// Create the middleware chain
			gzipMiddleware := GzipMiddleware(mockFS)
			handler := gzipMiddleware(nextHandler)
			handler.ServeHTTP(rr, req)

			// --- Assertions ---
			if rr.Code != tc.expectedStatus {
				t.Errorf("wrong status code: got %d, want %d", rr.Code, tc.expectedStatus)
			}

			// Check headers for gzipped responses
			if tc.expectedGzipped {
				if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
					t.Errorf("wrong Content-Encoding header: got %q, want %q", got, "gzip")
				}
				if got := rr.Header().Get("Vary"); got != "Accept-Encoding" {
					t.Errorf("wrong Vary header: got %q, want %q", got, "Accept-Encoding")
				}
			} else {
				if got := rr.Header().Get("Content-Encoding"); got != "" {
					t.Errorf("Content-Encoding header should be empty, but got %q", got)
				}
			}

			// Check content type, but only if we expect a successful response
			if tc.expectedStatus == http.StatusOK {
				if got := rr.Header().Get("Content-Type"); got != tc.expectedContentType {
					t.Errorf("wrong Content-Type header: got %q, want %q", got, tc.expectedContentType)
				}
			}

			// Check body content
			if !bytes.Equal(rr.Body.Bytes(), tc.expectedContent) {
				t.Errorf("wrong body content:\ngot:  %q\nwant: %q", rr.Body.Bytes(), tc.expectedContent)
			}
		})
	}

	t.Run("falls back when file is not seekable", func(t *testing.T) {
		// Create a mock FS with a file that will be made non-seekable
		gzData := newGzippedBytes(t, "some data")
		seekableFS := fstest.MapFS{
			"file.txt.gz": {Data: gzData},
		}

		// Wrap the seekable FS with our custom non-seekable wrapper
		nonSeekableTestFS := &nonSeekableFS{FS: seekableFS}

		req := httptest.NewRequest(http.MethodGet, "/file.txt", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()

		// A simple fallback that just writes "fallback"
		fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := io.WriteString(w, "fallback executed"); err != nil {
				t.Fatalf("fallback handler failed to write response: %v", err)
			}
		})

		middleware := GzipMiddleware(nonSeekableTestFS)
		handler := middleware(fallback)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("wrong status code: got %d, want %d", rr.Code, http.StatusOK)
		}
		if body := rr.Body.String(); body != "fallback executed" {
			t.Errorf("wrong body: got %q, want %q", body, "fallback executed")
		}
		if got := rr.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("Content-Encoding should be empty, but got %q", got)
		}
	})
}