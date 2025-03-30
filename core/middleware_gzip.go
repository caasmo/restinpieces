package core

// TODO
// If-Modified-Since
// For fully static assets (common with embedded files), you can simplify further:
// http.ServeContent(w, r, name, time.Time{}, f)
// This disables If-Modified-Since checks but improves performance.

import (
	"io"
	"io/fs"
	"mime"
	"time"
	"net/http"
	"path/filepath"
	"strings"
)

func (a *App) GzipMiddleware(fsys fs.FS, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip non-GET/HEAD requests immediately
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			slog.Debug("gzip middleware skipping non-GET/HEAD request", "method", r.Method)
			next.ServeHTTP(w, r)
			return
		}

		// Check Gzip support
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			// Attempt to serve precompressed version
			gzPath := r.URL.Path + ".gz"
			if f, err := fsys.Open(gzPath); err == nil {
				defer f.Close()
				slog.Debug("serving precompressed gzip file", "path", gzPath)

				// Set headers
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Add("Vary", "Accept-Encoding")
				
				// Get and set content type
				ext := filepath.Ext(r.URL.Path)
				if ct := mime.TypeByExtension(ext); ct != "" {
					w.Header().Set("Content-Type", ct)
				}

				// Serve directly using FileServerFS's underlying mechanisms
				http.ServeContent(w, r, r.URL.Path, time.Time{}, f.(io.ReadSeeker))
				return
			}
		}

		slog.Debug("falling back to regular handler", "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

