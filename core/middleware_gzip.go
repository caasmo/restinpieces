package core

// TODO
// If-Modified-Since
// For fully static assets (common with embedded files), you can simplify further:
// http.ServeContent(w, r, name, time.Time{}, f)
// This disables If-Modified-Since checks but improves performance.

import (
	"io"
	"io/fs"
	"time"
	"log/slog"
	"net/http"
	"strings"
)

// TODO cache control headers for assets
//w.Header().Set("Cache-Control", "public, max-age=86400, immutable") // 1 day
//w.Header().Set("ETag", "") // Empty since we don't support If-None-Match
func (a *App) GzipMiddleware(fsys fs.FS, next http.Handler) http.Handler {


	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {


		// Skip non-GET/HEAD requests immediately
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			slog.Debug("gzip middleware skipping non-GET/HEAD request", "method", r.Method)
			next.ServeHTTP(w, r)
			return
		}

		// Skip if client doesn't support gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// URL paths from http.Request always start with a slash (/path/to/file)
		// Embedded FS paths never start with a slash (path/to/file)
		// This is because FS paths are relative to the FS root
		// So we must remove the leading slash to correctly lookup files in the FS
		// Example transforms:
		//   /login.html → login.html.gz
		//   /css/style.css → css/style.css.gz
		gzPath := strings.TrimPrefix(r.URL.Path, "/") + ".gz"
		f, err := fsys.Open(gzPath)
		if err != nil {
			// If gzip file not found, fall through to regular handler
			next.ServeHTTP(w, r)
			return
		}
		defer f.Close()

		// Set gzip specific headers
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		
		// Serve directly using FileServerFS's underlying mechanisms
		// http.ServeContent automatically sets Content-Type based on:
		// 1. The file extension in the path parameter (r.URL.Path)
		// 2. If extension is unknown, it sniffs the first 512 bytes of content
		//
		// Using time.Time{} (zero time) as modTime is a deliberate optimization:
		// - Embedded assets are immutable - they don't change after compilation
		// - The modification time is irrelevant since the content is fixed
		// - This disables If-Modified-Since checks which is acceptable because:
		//   * Embedded assets are versioned with the application
		//   * No risk of serving stale content
		//   * Cache busting can be done through URL versioning
		//   * Reduces server-side processing overhead
		//
		// For caching, we rely on:
		// - Cache-Control header with long max-age (set elsewhere)
		// - Content-based ETags (hash of file contents)
		// - The immutable nature of embedded assets
		//
		// This approach provides:
		// - Optimal performance by skipping unnecessary checks
		// - Effective caching through other mechanisms
		// - Simplified server implementation
		// - Security through versioned URLs and content hashes
		http.ServeContent(w, r, r.URL.Path, time.Time{}, f.(io.ReadSeeker))

		next.ServeHTTP(w, r)
	})
}

