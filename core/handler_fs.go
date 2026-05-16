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
func FSHandler(fsys fs.FS, explicitPath string, compressionExt string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var action routeAction
		var basePath string

		// 1. Gatekeeper Logic
		if explicitPath != "" {
			action = actionServe
			basePath = explicitPath
		} else {
			action, basePath = resolveURL(r.URL.Path)
		}

		// 2. Execute Routing Instruction
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
			return
		}

		// 3. Resolve Compression
		openPath, encodingName := resolveCompression(basePath, compressionExt, r.Header.Get("Accept-Encoding"))

		// 4. Filesystem Operations
		// EXACTLY ONE FS OPEN (No fallback guesswork)
		f, err := fsys.Open(openPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = f.Close() }()

		// EXACTLY ONE FS STAT 
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

		// 5. Apply Headers & Serve
		if encodingName != "" {
			w.Header().Set("Content-Encoding", encodingName)
			w.Header().Set("Vary", "Accept-Encoding")
		}

		// Zero-time optimization prevents If-Modified-Since overhead.
		// CRITICAL: We pass `basePath` (e.g. "app.js") to ServeContent, NOT `openPath`. 
		// This forces Go to automatically set Content-Type: application/javascript instead 
		// of sniffing binary data and setting octet-stream.
		http.ServeContent(w, r, basePath, time.Time{}, seeker)
	}
}

// resolveURL translates a raw URL path into an actionable routing command 
// and a base filesystem path.
func resolveURL(requestPath string) (routeAction, string) {
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
	if !strings.HasSuffix(requestPath, "/") {
		return actionRedirect, ""
	}

	return actionServe, cleanPath + "/index.html"
}

// resolveCompression determines if a compressed version of the file should be served.
// It returns the physical path to open, and the Content-Encoding header value to set.
func resolveCompression(basePath, compressionExt, acceptEncoding string) (openPath string, encodingName string) {
	if compressionExt == "" {
		return basePath, ""
	}

	// Never compress HTML files
	if strings.HasSuffix(basePath, ".html") || strings.HasSuffix(basePath, ".htm") {
		return basePath, ""
	}

	// Map standard extensions to HTTP encoding names
	if compressionExt == ".gz" {
		encodingName = "gzip"
	} else if compressionExt == ".br" {
		encodingName = "br"
	} else {
		// Unknown compression extension
		return basePath, ""
	}

	// Check if the browser actually supports this encoding
	if strings.Contains(acceptEncoding, encodingName) {
		return basePath + compressionExt, encodingName
	}

	return basePath, ""
}
