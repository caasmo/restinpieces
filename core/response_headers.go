package core

import (
	"net/http"
)

var apiJsonDefaultHeaders = map[string]string{

	"Content-Type": "application/json; charset=utf-8",

	// Ensure the browser respects the declared content type strictly.
	// mitigate MIME-type sniffing attacks
	// browsers sometimes "sniff" or guess the content type of a resource based on its
	// actual content, rather than strictly adhering to the Content-Type header.
	// Attackers can exploit this by uploading malicious content.
	"X-Content-Type-Options": "nosniff",

	// The response must not be stored in any cache, anywhere, under any circumstances
	// no-store alone is enough to prevent all caching
	// no-cache and must-revalidate is just assurance if something downstream misinterprets no-store.
	"Cache-Control": "no-store, no-cache, must-revalidate",

	// Prevents the response from being embedded in an <iframe>, mitigating clickjacking attacks
	// Adds a layer of defense against obscure misuse
	"X-Frame-Options": "DENY",

	// Controls cross-origin resource sharing (CORS)
	// be restrictive, most restrictive is not to have it, same domain as api endpoints
	// TODO configurable
	//"Access-Control-Allow-Origin": "*",

	// HSTS TODO configurable  based on server are we under TLS terminating proxy
	//"Strict-Transport-Security": "max-age=31536000",
}

// headerCacheStatic defines cache headers for immutable static assets (CSS, JS, images).
// Assumes filename-based versioning for cache busting.
var headerCacheStatic = map[string]string{
	// - public: Allows caching by intermediate proxies and browsers.
	// - max-age=31536000: Cache for 1 year.
	// - immutable: Indicates the file content will never change. Browsers
	//              will not even attempt to revalidate it, providing maximum
	//              caching efficiency. Relies entirely on filename versioning.
	"Cache-Control": "public, max-age=31536000, immutable",
}

// headerCacheStaticHtml defines cache headers for HTML entry point files.
var headerCacheStaticHtml = map[string]string{
	// - public: Allows caching by intermediate proxies and browsers.
	// - no-cache: Requires the cache to revalidate with the origin server
	//             before using a cached response. Ensures the latest HTML
	//             (with potentially updated asset links) is served.
	"Cache-Control": "public, no-cache",
}

// TODO
var htmlHeaders = map[string]string{

	// CSP governs browser behavior for resources loaded as part of rendering a document
	// Prevents cross-site scripting (XSS) attacks by controlling which resources can be loaded.
	// means: “By default, only load resources from this server’s origin, nothing external.”
	// Unnecessary for pure API servers since they don't serve HTML/JavaScript
	"Content-Security-Policy": "default-src 'self'",

	// mitigate reflected XSS attacks: malicious scripts are injected into a
	// page via user input (e.g., query parameters, form data) and then
	// "reflected" back to the user in the server’s response.
	// 1: Enables the browser’s XSS filter
	// mode=block: Instructs the browser to block the entire page if an XSS attack is detected
	//
	// Modern browsers (post-2019 Chrome, Edge, etc.) ignore this header, favoring Content Security Policy (CSP)
	// this header is mostly a legacy tool
	// Optional for API servers, but no harm
	//"X-XSS-Protection":           "1; mode=block",
}

// ApplyHeaders sets all headers from a map to the response writer
func setHeaders(w http.ResponseWriter, headers map[string]string) {
	for key, value := range headers {
		w.Header()[key] = []string{value}
	}
}
