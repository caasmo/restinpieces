package core

import (
	"io/fs"
	"net/http"
)

// MPAShellHandler serves a pre-built HTML shell file for client-rendered routes
// in Multi-Page Applications (MPAs) and Single-Page Applications (SPAs).
//
// # What Is a Shell File
//
// A shell is a minimal, static HTML file produced by your JavaScript bundler
// (Vite, Webpack, etc.). It contains no meaningful content on its own — just
// the <script> and <link> tags that bootstrap your JavaScript framework
// (React, Vue, Svelte, etc.). On load, the framework takes over, calls your
// JSON API, and renders the page entirely in the browser.
//
// Example shell (dist/books/index.html produced by Vite):
//
//	<!doctype html>
//	<html>
//	  <head>
//	    <link rel="stylesheet" href="/dist/assets/main-DiwrgTda.css">
//	  </head>
//	  <body>
//	    <div id="root"></div>
//	    <script type="module" src="/dist/assets/main-BgFVfQP3.js"></script>
//	  </body>
//	</html>
//
// # When to Use This Handler
//
// Use MPAShellHandler when:
//   - The route is dynamic (e.g. "/books/{id}/sentences/{idx}")
//   - The response is always the same HTML file regardless of URL parameters
//   - Data fetching and rendering happen entirely in the browser via your JSON API
//   - You accept the initial render being empty until JavaScript executes
//     (blank flash or loading spinner before content appears)
//
// # When NOT to Use This Handler
//
// Do not use MPAShellHandler when the page must arrive with data already embedded.
// If you need to eliminate the blank-flash and have content immediately visible
// (better SEO, faster perceived load, no flicker), implement a custom handler
// that uses [ssr.Marshal] to inject data into the template before sending:
//
//	func (a *App) BookPageHandler(templatePath string) http.HandlerFunc {
//	    templateBytes, err := fs.ReadFile(a.fs, templatePath)
//	    if err != nil {
//	        panic("missing SSR template: " + templatePath)
//	    }
//	    return func(w http.ResponseWriter, r *http.Request) {
//	        bookId := a.Router().Param(r, "bookId")
//	        book, err := a.db.GetBook(bookId)
//	        // ... error handling ...
//	        output, err := ssr.Marshal(templateBytes, map[string]any{
//	            "book": book,
//	        }, nonce)
//	        // ... write response ...
//	    }
//	}
//
// # Parameters
//
//   - fsys (fs.FS): The underlying filesystem. Typically an embed.FS or os.DirFS.
//     With embed.FS (recommended), the file is compiled into the binary and
//     fs.ReadFile is a pure memory read with no I/O cost.
//
//   - shellPath (string): Clean, relative path to the shell HTML file within fsys.
//     Must point to a file, not a directory (e.g. "books/index.html").
//     The path is validated at startup: if the file is missing, the handler
//     panics immediately rather than returning 500s at request time.
//
// # Startup Validation and Caching
//
// The file is read once when routing is wired and its bytes are held in memory.
// All subsequent requests are served directly from that in-memory buffer — zero
// filesystem activity at request time, regardless of whether fsys is an embed.FS
// or an os.DirFS. If shellPath is not found, the handler panics immediately:
// a missing shell means the deployment is broken and should fail at startup,
// not silently return 500s to users.
//
// # Usage
//
// In your router, pair dynamic URL patterns with their corresponding shell:
//
//	a.Router().Register(r.Chains{
//	    // Client-rendered: JS fetches data from /api/books/{bookId} after load
//	    "GET /books/{bookId}": r.NewChain(
//	        core.MPAShellHandler(a.FS(), "books/index.html"),
//	    ).WithMiddleware(a.StaticAssetHeadersMiddleware),
//
//	    // Client-rendered: JS fetches from /api/books/{bookId}/sentences/{sentenceIndex}
//	    "GET /books/{bookId}/sentences/{sentenceIndex}": r.NewChain(
//	        core.MPAShellHandler(a.FS(), "sentences/index.html"),
//	    ).WithMiddleware(a.StaticAssetHeadersMiddleware),
//
//	    // SSR route: data injected server-side, no blank flash — custom handler instead
//	    "GET /books/{bookId}/sentences/{sentenceIndex}/tokens": r.NewChain(
//	        a.SentenceNlpSsrHandler("internal/sentences-nlp/index.html"),
//	    ).WithMiddleware(a.SSRHeadersMiddleware),
//
//	    // Catch-all: static assets (js, css, images) with gzip compression
//	    "/": r.NewChain(
//	        core.FSHandler(a.FS(), "", core.CompressExtGzip),
//	    ).WithMiddleware(a.StaticAssetHeadersMiddleware),
//	})
func MPAShellHandler(fsys fs.FS, shellPath string) http.HandlerFunc {
	// Read ONCE at startup. Panic is correct: a missing shell is a broken
	// deployment that should fail immediately, not silently at request time.
	// Works correctly for both embed.FS (memory read) and os.DirFS (disk read).
	content, err := fs.ReadFile(fsys, shellPath)
	if err != nil {
		panic("MPAShellHandler: missing shell file: " + shellPath + ": " + err.Error())
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
	}
}
