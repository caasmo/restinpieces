package core

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestFSHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":                {Data: []byte("root index")},
		"about/index.html":          {Data: []byte("about index")},
		"app.js":                    {Data: []byte("console.log('hi')")},
		"assets/style.css":          {Data: []byte("body { color: red; }")},
		"custom.html":               {Data: []byte("custom page")},
		"no-extension":              {Data: []byte("i have no extension")},
		"dir-only/somefile.txt":     {Data: []byte("some text")},
	}

	tests := []struct {
		name           string
		explicitPath   string
		requestPath    string
		expectedStatus int
		expectedBody   string
		expectedLoc    string // for redirects
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
			name:           "root with explicit path",
			explicitPath:   "index.html",
			requestPath:    "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "root index",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			rr := httptest.NewRecorder()

			handler := FSHandler(fsys, tt.explicitPath)
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
	// http.ServeContent usually handles other methods like HEAD,
	// but let's see how FSHandler handles non-GET.
	fsys := fstest.MapFS{
		"index.html": {Data: []byte("root index")},
	}
	
	t.Run("POST request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rr := httptest.NewRecorder()
		handler := FSHandler(fsys, "")
		handler.ServeHTTP(rr, req)
		
		// http.ServeContent might allow POST but it depends on implementation.
		// Actually FSHandler doesn't restrict method.
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
	
	handler := FSHandler(fsys, "")
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
	
	handler := FSHandler(fsys, "")
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
	
	handler := FSHandler(fsys, "")
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
		// Even if we try to trick it with an extension, 
		// if it's a directory on FS, it should 404.
		// (fstest doesn't easily support making a file named 'dir.js' a directory,
		// but we can test the stat.IsDir check)
		
		req := httptest.NewRequest(http.MethodGet, "/dir", nil)
		rr := httptest.NewRecorder()
		handler := FSHandler(fsys, "")
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
		// If a directory is named "script.js", the handler might try to serve it as a file.
		// The stat.IsDir check should catch this and return 404.
		fsys := fstest.MapFS{
			"script.js/index.html": {Data: []byte("index")},
		}

		req := httptest.NewRequest(http.MethodGet, "/script.js", nil)
		rr := httptest.NewRecorder()
		handler := FSHandler(fsys, "")
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404 for directory with file extension, got %d", rr.Code)
		}
	})
}
