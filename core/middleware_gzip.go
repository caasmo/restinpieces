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
	"log/slog"
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

		slog.Debug("trying to serve", "path", r.URL.Path)
		// Check Gzip support
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			// Attempt to serve precompressed version
			slog.Debug("found header", "path", r.URL.Path)
			// Remove leading slash to match embedded FS paths
			gzPath := strings.TrimPrefix(r.URL.Path, "/") + ".gz"
			slog.Debug("attempting to open gzip file", "path", gzPath)
			f, err := fsys.Open(gzPath)
			if err != nil {
				slog.Debug("failed to open gzip file", "path", gzPath, "error", err)
				// For debugging, list all files in the FS
				fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() {
						slog.Debug("available file in FS", "path", path)
					}
					return nil
				})
			} else {
				defer f.Close()
				slog.Debug("successfully opened gzip file", "path", gzPath)

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

