package core

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// FSHandler is a highly optimized static file handler designed specifically for 
// Multi-Page Applications (MPAs) with dynamic backend routing.
//
// WHY NOT http.FileServerFS?
// 1. Dynamic Pages: http.FileServerFS strictly maps the HTTP Request URL to the filesystem path. 
//    We need to map dynamic RESTful URLs (e.g., "GET /books/{id}/sentences/{idx}") to specific 
//    static HTML bundles (e.g., "sentence-nlp/index.html").
// 2. Performance: http.FileServerFS performs 2 to 4 filesystem operations (Open, Stat, Stat) 
//    to resolve directories to index.html and handle redirects.
//
// OUR PERFORMANCE GUARANTEE:
// This handler guarantees exactly ONE filesystem Open and ONE filesystem Stat per successful 
// request, completely avoiding TOCTOU (Time-of-Check to Time-of-Use) issues.
//
// THE CLIENT CONTRACT (SDK/Frontend):
// To achieve this performance, we enforce a strict in-memory contract based on file extensions:
// - Files: The JS SDK/HTML must request actual files with their extensions (e.g., "/app.js").
// - Directories (MPA Routes): Any URL path without an extension is assumed to be an MPA route, 
//   and we append "/index.html" strictly in-memory. Extensionless files are not supported 
//   via the catch-all (map them explicitly if needed).
//
// EXAMPLES OF EXECUTION:
// 
// 1. Dynamic Route (Explicit Path Provided)
//    Call: PageHandler(fsys, "sentence-nlp/index.html")
//    Req:  GET /books/123/sentences/5
//    Exec: Opens "sentence-nlp/index.html" directly. (1 Open, 1 Stat).
//
// 2. Static Assets (Empty Explicit Path, resolved from URL)
//    Call: PageHandler(fsys, "")
//    Req:  GET /dist/app.js
//    Exec: Has ".js" extension. Opens "dist/app.js". (1 Open, 1 Stat).
//
// 3. Catch-All MPA Route without Trailing Slash
//    Call: PageHandler(fsys, "")
//    Req:  GET /about
//    Exec: No extension. Assumed MPA directory. Missing trailing slash. 
//          Returns HTTP 301 Redirect to "/about/". (0 FS Opens!).
//          Why? If we silently serve "about/index.html", the browser calculates relative 
//          HTML links incorrectly (e.g. <img src="./logo.png"> looks at the root instead of /about/).
//
// 4. Catch-All MPA Route with Trailing Slash
//    Call: PageHandler(fsys, "")
//    Req:  GET /about/
//    Exec: No extension. Has trailing slash. Appends index.html. 
//          Opens "about/index.html". (1 Open, 1 Stat).
func FSHandler(fsys fs.FS, explicitPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := explicitPath

		// If no explicit path is given, we resolve it from the URL purely in-memory
		// to avoid expensive filesystem guesswork.
		if target == "" {
			cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")

			if cleanPath == "" || cleanPath == "." {
				// Root route -> always index.html
				target = "index.html"
			} else if path.Ext(cleanPath) == "" {
				// NO EXTENSION: Assume it is an MPA route/directory.
				
				// Fix the Browser Relative Path Redirection Issue purely in memory.
				// If we don't force a trailing slash, relative links in the HTML will break.
				if !strings.HasSuffix(r.URL.Path, "/") {
					http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
					return // Executed with 0 filesystem calls.
				}
				
				// Has trailing slash, safe to append index.html in-memory.
				target = cleanPath + "/index.html"
			} else {
				// HAS EXTENSION (e.g. .css, .js, .png): Assume it's a direct file asset.
				target = cleanPath
			}
		}

		// EXACTLY ONE FS OPEN
		f, err := fsys.Open(target)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()

		// EXACTLY ONE FS STAT 
		// We do this after Open() to avoid TOCTOU bugs. We also use this to ensure our 
		// in-memory guess didn't accidentally target a real directory (which would leak or panic).
		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			http.NotFound(w, r)
			return
		}

		// Type assert the file to a ReadSeeker, which http.ServeContent requires.
		// standard os.File and embed.FS files both implement this.
		seeker, ok := f.(io.ReadSeeker)
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Using time.Time{} (zero time) as modTime is a deliberate optimization:
		// - Embedded assets are immutable - they don't change after compilation
		// - The modification time is irrelevant since the content is fixed
		// - This disables If-Modified-Since checks which is acceptable because:
		//   * Reduces server-side processing overhead
		http.ServeContent(w, r, target, time.Time{}, seeker)
	}
}
