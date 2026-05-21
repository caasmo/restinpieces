package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestMPAShellHandler(t *testing.T) {
	content := "<!DOCTYPE html><html><body>Shell</body></html>"
	fsys := fstest.MapFS{
		"dist/index.html": {Data: []byte(content)},
	}

	t.Run("success", func(t *testing.T) {
		handler := MPAShellHandler(fsys, "dist/index.html")
		req := httptest.NewRequest(http.MethodGet, "/any-path", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
		if contentType := rr.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
			t.Errorf("expected Content-Type text/html; charset=utf-8, got %q", contentType)
		}
		if rr.Body.String() != content {
			t.Errorf("expected body %q, got %q", content, rr.Body.String())
		}
	})

	t.Run("HEAD request", func(t *testing.T) {
		handler := MPAShellHandler(fsys, "dist/index.html")
		req := httptest.NewRequest(http.MethodHead, "/any-path", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("Method Not Allowed", func(t *testing.T) {
		handler := MPAShellHandler(fsys, "dist/index.html")
		methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

		for _, method := range methods {
			t.Run(method, func(t *testing.T) {
				req := httptest.NewRequest(method, "/any-path", nil)
				rr := httptest.NewRecorder()

				handler.ServeHTTP(rr, req)

				if rr.Code != http.StatusMethodNotAllowed {
					t.Errorf("expected status 405, got %d", rr.Code)
				}
				if allow := rr.Header().Get("Allow"); allow != "GET, HEAD" {
					t.Errorf("expected Allow header 'GET, HEAD', got %q", allow)
				}
			})
		}
	})

	t.Run("panic on missing file", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic for missing shell file, but it did not panic")
			}
		}()

		_ = MPAShellHandler(fsys, "missing.html")
	})
}
