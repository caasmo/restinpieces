package core

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// Helper to create a gzipped version of a string.
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

func TestFSHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":            {Data: []byte("root index")},
		"about/index.html":      {Data: []byte("about index")},
		"app.js":                {Data: []byte("console.log('hi')")},
		"app.js.gz":             {Data: newGzippedBytes(t, "console.log('hi')")},
		"style.css.br":          {Data: []byte("body { color: red; } br")},
		"assets/style.css":      {Data: []byte("body { color: red; }")},
		"custom.html":           {Data: []byte("custom page")},
		"no-extension":          {Data: []byte("i have no extension")},
		"_private.txt":          {Data: []byte("secret")},
		"dir/_hidden/file.txt":  {Data: []byte("hidden")},
		"dir-only/somefile.txt": {Data: []byte("some text")},
	}

	tests := []struct {
		name            string
		explicitPath    string
		compressionExt  string
		requestPath     string
		requestHeaders  map[string]string
		expectedStatus  int
		expectedBody    string
		expectedLoc     string            // for redirects
		expectedHeaders map[string]string // for other headers
	}{
		{
			name:           "root path",
			requestPath:    "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "root index",
		},
		{
			name:           "direct file",
			requestPath:    "/app.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "console.log('hi')",
		},
		{
			name:           "nested file",
			requestPath:    "/assets/style.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "body { color: red; }",
		},
		{
			name:           "directory redirect",
			requestPath:    "/about",
			expectedStatus: http.StatusMovedPermanently,
			expectedLoc:    "/about/",
		},
		{
			name:           "directory with trailing slash",
			requestPath:    "/about/",
			expectedStatus: http.StatusOK,
			expectedBody:   "about index",
		},
		{
			name:           "query string preservation on redirect",
			requestPath:    "/about?foo=bar",
			expectedStatus: http.StatusMovedPermanently,
			expectedLoc:    "/about/?foo=bar",
		},
		{
			name:           "explicit path ignores request path",
			explicitPath:   "custom.html",
			requestPath:    "/some/random/path",
			expectedStatus: http.StatusOK,
			expectedBody:   "custom page",
		},
		{
			name:           "non-existent file",
			requestPath:    "/missing.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "extensionless file treated as MPA route (redirect)",
			requestPath:    "/no-extension",
			expectedStatus: http.StatusMovedPermanently,
			expectedLoc:    "/no-extension/",
		},
		{
			name:           "extensionless file treated as MPA route (not found after redirect)",
			requestPath:    "/no-extension/",
			expectedStatus: http.StatusNotFound, // because it looks for no-extension/index.html
		},
		{
			name:           "accessing directory directly",
			requestPath:    "/about/index.html",
			expectedStatus: http.StatusOK,
			expectedBody:   "about index",
		},
		{
			name:           "path cleaning",
			requestPath:    "/assets/../app.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "console.log('hi')",
		},
		{
			name:           "security: path traversal to root",
			requestPath:    "/../../",
			expectedStatus: http.StatusOK,
			expectedBody:   "root index", // cleaned to / -> index.html
		},
		{
			name:           "security: path traversal above root",
			requestPath:    "/../app.js",
			expectedStatus: http.StatusOK,
			expectedBody:   "console.log('hi')", // cleaned to /app.js
		},
		{
			name:           "multiple slashes",
			requestPath:    "///assets//style.css",
			expectedStatus: http.StatusOK,
			expectedBody:   "body { color: red; }",
		},
		{
			name:           "private file (underscore prefix)",
			requestPath:    "/_private.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "private directory (underscore prefix)",
			requestPath:    "/dir/_hidden/file.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "gzip compression: success",
			compressionExt: ".gz",
			requestPath:    "/app.js",
			requestHeaders: map[string]string{"Accept-Encoding": "gzip"},
			expectedStatus: http.StatusOK,
			expectedBody:   string(newGzippedBytes(t, "console.log('hi')")),
			expectedHeaders: map[string]string{
				"Content-Encoding": "gzip",
				"Vary":             "Accept-Encoding",
				"Content-Type":     "text/javascript; charset=utf-8",
			},
		},
		{
			name:           "gzip compression: zero fallback (missing file)",
			compressionExt: ".gz",
			requestPath:    "/assets/style.css",
			requestHeaders: map[string]string{"Accept-Encoding": "gzip"},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "gzip compression: no client support",
			compressionExt: ".gz",
			requestPath:    "/app.js",
			requestHeaders: map[string]string{"Accept-Encoding": "identity"},
			expectedStatus: http.StatusOK,
			expectedBody:   "console.log('hi')",
		},
		{
			name:           "gzip compression: HTML exclusion",
			compressionExt: ".gz",
			requestPath:    "/index.html",
			requestHeaders: map[string]string{"Accept-Encoding": "gzip"},
			expectedStatus: http.StatusOK,
			expectedBody:   "root index",
			expectedHeaders: map[string]string{
				"Content-Encoding": "",
			},
		},
		{
			name:           "br compression: success",
			compressionExt: ".br",
			requestPath:    "/style.css", // note: style.css doesn't exist, but style.css.br does. Wait, resolveURL will map style.css to style.css.
			// Actually if style.css doesn't exist, it should 404.
			// Let's use app.js but with .br if I had it.
			// The logic is: basePath = style.css, compressionExt = .br, openPath = style.css.br
			requestHeaders: map[string]string{"Accept-Encoding": "br"},
			expectedStatus: http.StatusOK,
			expectedBody:   "body { color: red; } br",
			expectedHeaders: map[string]string{
				"Content-Encoding": "br",
				"Vary":             "Accept-Encoding",
				"Content-Type":     "text/css; charset=utf-8",
			},
		},
		{
			name:           "unsupported compression extension",
			compressionExt: ".zip",
			requestPath:    "/app.js",
			requestHeaders: map[string]string{"Accept-Encoding": "zip"},
			expectedStatus: http.StatusOK,
			expectedBody:   "console.log('hi')",
		},
		{
			name:           "htm exclusion",
			compressionExt: ".gz",
			requestPath:    "/custom.htm",
			expectedStatus: http.StatusOK,
			expectedBody:   "htm content",
		},
	}

	fsys["custom.htm"] = &fstest.MapFile{Data: []byte("htm content")}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			for k, v := range tt.requestHeaders {
				req.Header.Set(k, v)
			}
			rr := httptest.NewRecorder()

			handler := FSHandler(fsys, tt.explicitPath, tt.compressionExt)
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedLoc != "" {
				loc := rr.Header().Get("Location")
				if loc != tt.expectedLoc {
					t.Errorf("expected location %q, got %q", tt.expectedLoc, loc)
				}
			}

			for k, v := range tt.expectedHeaders {
				got := rr.Header().Get(k)
				if got != v {
					t.Errorf("expected header %q to be %q, got %q", k, v, got)
				}
			}

			if tt.expectedBody != "" {
				body := rr.Body.String()
				if body != tt.expectedBody {
					t.Errorf("expected body %q, got %q", tt.expectedBody, body)
				}
			}
		})
	}
}

func TestFSHandler_MethodNotAllowed(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": {Data: []byte("root index")},
	}

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			rr := httptest.NewRecorder()
			handler := FSHandler(fsys, "", "")
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
			}

			if allow := rr.Header().Get("Allow"); allow != "GET, HEAD" {
				t.Errorf("expected Allow header to be 'GET, HEAD', got %q", allow)
			}
		})
	}

	t.Run("HEAD request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/", nil)
		rr := httptest.NewRecorder()
		handler := FSHandler(fsys, "", "")
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

func TestFSHandler_Range(t *testing.T) {
	fsys := fstest.MapFS{
		"large.txt": {Data: []byte("0123456789")},
	}

	req := httptest.NewRequest(http.MethodGet, "/large.txt", nil)
	req.Header.Set("Range", "bytes=2-5")
	rr := httptest.NewRecorder()

	handler := FSHandler(fsys, "", "")
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Errorf("expected status 206, got %d", rr.Code)
	}

	expected := "2345"
	if rr.Body.String() != expected {
		t.Errorf("expected body %q, got %q", expected, rr.Body.String())
	}
}

func TestFSHandler_NoLastModified(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": {Data: []byte("root index")},
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler := FSHandler(fsys, "", "")
	handler.ServeHTTP(rr, req)

	if lm := rr.Header().Get("Last-Modified"); lm != "" {
		t.Errorf("expected no Last-Modified header, got %q", lm)
	}
}

type mockFileNoSeeker struct {
	fs.File
}

func (m *mockFileNoSeeker) Stat() (fs.FileInfo, error) {
	return m.File.Stat()
}

type mockFSNoSeeker struct {
	fstest.MapFS
}

func (m *mockFSNoSeeker) Open(name string) (fs.File, error) {
	f, err := m.MapFS.Open(name)
	if err != nil {
		return nil, err
	}
	return &mockFileNoSeeker{File: f}, nil
}

func TestFSHandler_InternalError(t *testing.T) {
	// A file that doesn't implement io.ReadSeeker should trigger 500
	fsys := &mockFSNoSeeker{
		MapFS: fstest.MapFS{
			"index.html": {Data: []byte("root index")},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler := FSHandler(fsys, "", "")
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestFSHandler_DirectorySafety(t *testing.T) {
	fsys := fstest.MapFS{
		"dir/index.html": {Data: []byte("index")},
	}

	t.Run("cannot serve directory as file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dir", nil)
		rr := httptest.NewRecorder()
		handler := FSHandler(fsys, "", "")
		handler.ServeHTTP(rr, req)

		// /dir -> redirects to /dir/
		if rr.Code != http.StatusMovedPermanently {
			t.Errorf("expected redirect for directory, got %d", rr.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/dir/", nil)
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		// /dir/ -> serves dir/index.html
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 for index.html, got %d", rr.Code)
		}
	})

	t.Run("file extension on directory name", func(t *testing.T) {
		fsys := fstest.MapFS{
			"script.js/index.html": {Data: []byte("index")},
		}

		req := httptest.NewRequest(http.MethodGet, "/script.js", nil)
		rr := httptest.NewRecorder()
		handler := FSHandler(fsys, "", "")
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404 for directory with file extension, got %d", rr.Code)
		}
	})
}

