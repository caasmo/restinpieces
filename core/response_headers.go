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
var headersCacheStatic = map[string]string{
	// - public: Allows caching by intermediate proxies and browsers.
	// - max-age=31536000: Cache for 1 year.
	// - immutable: Indicates the file content will never change. Browsers
	//              will not even attempt to revalidate it, providing maximum
	//              caching efficiency. Relies entirely on filename versioning.
	"Cache-Control": "public, max-age=31536000, immutable",
}

// headerCacheStaticHtml defines cache headers for HTML entry point files.
var headersCacheStaticHtml = map[string]string{
	// - public: Allows caching by intermediate proxies and browsers.
	// - no-cache: Requires the cache to revalidate with the origin server
	//             before using a cached response. Ensures the latest HTML
	//             (with potentially updated asset links) is served.
	"Cache-Control": "public, no-cache",
}

// headersCacheStaticDev defines cache headers for static assets during development.
// The primary goal is to prevent any caching to ensure changes are reflected immediately.
var headersCacheStaticDev = map[string]string{
	// - no-store: The most restrictive directive. Instructs caches (browser, intermediate)
	//             not to store the response under any circumstances.
	// - no-cache: While technically redundant with no-store, it's often included
	//             for broader compatibility with older caches that might misinterpret no-store.
	//             It forces revalidation, but no-store prevents storage altogether.
	// - must-revalidate: Also redundant with no-store, but reinforces that caches
	//                    must revalidate stale resources (which won't happen if they aren't stored).
	"Cache-Control": "no-store, no-cache, must-revalidate",
	// Pragma: no-cache is an HTTP/1.0 header sometimes included for compatibility
	// with very old caches, though Cache-Control is the modern standard.
	"Pragma": "no-cache",
	// Expires: 0 is another way to indicate that the content is already expired,
	// used by older caches.
	"Expires": "0",
}


// headersSecurityStaticHtml defines security-related headers specifically for HTML documents.
var headersSecurityStaticHtml = map[string]string{
	// Content-Security-Policy (CSP) governs browser behavior for resources loaded as part of rendering a document.
	// Prevents cross-site scripting (XSS) attacks by controlling which resources can be loaded.
	// 'default-src 'self'': By default, only load resources (scripts, styles, images, fonts, etc.)
	//                      from the same origin as the HTML document. This is a strong baseline.
	//                      Needs adjustment if you load resources from CDNs or other domains.
	"Content-Security-Policy": "default-src 'self'",

	// Other security headers like X-Frame-Options, Referrer-Policy, Permissions-Policy
	// could be added here later if needed for HTML responses.

	// Example of a previously considered header (X-XSS-Protection):
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
