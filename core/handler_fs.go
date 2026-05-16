package core

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// routeAction defines the explicit instruction returned by the URL resolver.
type routeAction int

const (
	actionServe routeAction = iota
	actionRedirect
	actionNotFound
)

// FSHandler is a highly optimized static file handler designed specifically for 
// Multi-Page Applications (MPAs) with dynamic backend routing.
//
// OUR PERFORMANCE GUARANTEE:
// This handler guarantees exactly ONE filesystem Open and ONE filesystem Stat per successful 
// request, completely avoiding TOCTOU (Time-of-Check to Time-of-Use) issues.
//
// CONVENTIONS & RULES:
// - Explicit Paths: Bypasses all URL resolution and serves the exact file requested.
// - Files: Any URL with an extension (e.g., "/app.js") is served directly.
// - Directories (MPA Routes): URLs without extensions are treated as directories. 
//   If the trailing slash is missing, it issues a 301 Redirect. If present, it appends "index.html".
// - Private Paths: Any file or directory starting with an underscore (e.g., "_search" or "dir/_hidden.js") 
//   is private and will return a 404 Not Found.
func FSHandler(fsys fs.FS, explicitPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var action routeAction
		var filePath string

		// 1. Gatekeeper Logic
		// If an explicit override is provided, bypass all resolution logic.
		if explicitPath != "" {
			action = actionServe
			filePath = explicitPath
		} else {
			action, filePath = resolveURL(r.URL.Path)
		}

		// 2. Execute Instruction
		switch action {
		case actionNotFound:
			http.NotFound(w, r)
			return

		case actionRedirect:
			// Fix the Browser Relative Path Redirection Issue.
			// Preserve query string during the trailing slash redirect.
			redirectURL := r.URL.Path + "/"
			if r.URL.RawQuery != "" {
				redirectURL += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, redirectURL, http.StatusMovedPermanently)
			return // Executed with 0 filesystem calls.

		case actionServe:
			// Proceed to filesystem operations below.
		}

		// 3. Filesystem Operations
		// EXACTLY ONE FS OPEN
		f, err := fsys.Open(filePath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = f.Close() }()

		// EXACTLY ONE FS STAT 
		// Done after Open() to avoid TOCTOU bugs. Ensures we didn't target a directory.
		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			http.NotFound(w, r)
			return
		}

		// Type assert the file to a ReadSeeker, which http.ServeContent requires.
		seeker, ok := f.(io.ReadSeeker)
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Zero-time optimization prevents If-Modified-Since overhead for embedded assets.
		http.ServeContent(w, r, filePath, time.Time{}, seeker)
	}
}

// resolveURL is a pure, deterministic function that translates a raw URL path 
// into an actionable routing command and a filesystem path.
// It has no knowledge of the HTTP request, response, or filesystem state.
func resolveURL(requestPath string) (routeAction, string) {
	// Clean the path to standard format and remove the leading slash for fs.FS
	cleanPath := strings.TrimPrefix(path.Clean(requestPath), "/")

	// PRIVATE PATH RULE: If any segment starts with an underscore, it is forbidden.
	if strings.HasPrefix(cleanPath, "_") || strings.Contains(cleanPath, "/_") {
		return actionNotFound, ""
	}

	// ROOT RULE: Always maps to index.html
	if cleanPath == "" || cleanPath == "." {
		return actionServe, "index.html"
	}

	// EXTENSION RULE: If it has an extension, treat it as a direct asset request
	if path.Ext(cleanPath) != "" {
		return actionServe, cleanPath
	}

	// MPA ROUTE RULE (No Extension):
	// If the original request lacks a trailing slash, it must be redirected 
	// so the browser resolves relative assets (like ./logo.png) correctly.
	if !strings.HasSuffix(requestPath, "/") {
		return actionRedirect, ""
	}

	// It is an MPA route with a valid trailing slash. Append index.html.
	return actionServe, cleanPath + "/index.html"
}
