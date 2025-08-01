package core

// TODO
// If-Modified-Since
// For fully static assets (common with embedded files), you can simplify further:
// http.ServeContent(w, r, name, time.Time{}, f)
// This disables If-Modified-Since checks but improves performance.

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// TODO cache control headers for assets
// w.Header().Set("Cache-Control", "public, max-age=86400, immutable") // 1 day
// w.Header().Set("ETag", "") // Empty since we don't support If-None-Match
// For caching, we shoudl rely on:
// - Cache-Control header with long max-age (set elsewhere)
// - Content-based ETags (hash of file contents)
// - The immutable nature of embedded assets
// TODO versioning
//   - Embedded assets are versioned with the application
//   - No risk of serving stale content
//   - Cache busting can be done through URL versioning
//
// TODO no dependency on app.
func GzipMiddleware(fsys fs.FS) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Skip non-GET/HEAD requests immediately
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
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
				// If gzipped file doesn't exist, or another error occurs (e.g. permissions),
				// fall through to the next handler which will serve the uncompressed version.
				// A logger would be useful here for non-ErrNotExist cases.
				next.ServeHTTP(w, r)
				return
			}
			defer func() { _ = f.Close() }()

			// Check if the file implements io.ReadSeeker. While files from standard library
			// fs.FS implementations (like embed.FS, os.DirFS) typically do, custom implementations
			// might not. http.ServeContent works most efficiently with io.ReadSeeker (e.g., for Range requests).
			// This check ensures robustness against different fs.FS sources.
			seeker, ok := f.(io.ReadSeeker)
			if !ok {
				// Fall back to the next handler as we cannot efficiently serve this file.
				next.ServeHTTP(w, r)
				return
			}

			// Set gzip specific headers
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")

			// Serve directly using FileServerFS's underlying mechanisms
			// http.ServeContent automatically sets Content-Type based on:
			// 1. The file extension in the path parameter (r.URL.Path)
			// 2. If extension is unknown, it sniffs the first 512 bytes of content
			//
			// Using time.Time{} (zero time) as modTime is a deliberate optimization:
			// - Embedded assets are immutable - they don't change after compilation
			// - The modification time is irrelevant since the content is fixed
			// - This disables If-Modified-Since checks which is acceptable because:
			//   * Reduces server-side processing overhead
			http.ServeContent(w, r, r.URL.Path, time.Time{}, seeker)
		})
	}
}
