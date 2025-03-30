package core

import (
	"net/http"
)

// TODO consiten name
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

// headersSecurityStaticHtml defines security-related headers specifically for HTML documents.
var headersSecurityStaticHtml = map[string]string{
	// Content-Security-Policy (CSP) governs browser behavior for resources loaded as part of rendering a document.
	// Prevents cross-site scripting (XSS) attacks by controlling which resources can be loaded.
	// 'default-src 'self'': By default, only load resources (scripts, styles, images, fonts, etc.)
	//                      from the same origin as the HTML document. This is a strong baseline.
	//                      Needs adjustment if you load resources from CDNs or other domains.
	//
	// 'default-src 'self'' disable inline scripts and style! like:
	//
	// <style>body { color: red }</style>
	// <script>alert('hi');</script>
	//
	// also no
    //    <button onclick="window.location.href='/verify-email.html'" style="background-color: #4caf50" >
	//
	// but allows:
	//
	// <link rel="stylesheet" href="/styles.css">
	// <script src="/script.js"></script>
	//
	// Using external CSS/JS files via <link> and <script> is more secure than inline code
	// Inline scripts/styles (<script>...</script>, <style>...</style>, or
	// inline event handlers like onclick="...") are high-risk vectors for XSS
	// attacks. If an attacker injects malicious code into dynamically
	// generated content (e.g., user comments), the browser will execute it.
	// External files avoid this because they’re static assets served from your
	// domain, which attackers can’t easily modify unless they compromise your
	// server.
	//
	// It also enables Subresource Integrity (SRI). External files can use the
	// integrity attribute to cryptographically verify that the file hasn’t
	// been tampered with (e.g., a compromised CDN or MITM attack). Inline code
	// lacks this safeguard
	// <script src="/script.js" integrity="sha256-..."></script>
	//
	// CSP wants you to:
	// - Serve all code as external files from trusted origins ('self', trusted CDNs).
	// - Avoid inline code entirely (no 'unsafe-inline' exceptions).
	// - Use SRI to ensure file integrity
	"Content-Security-Policy": "default-src 'self'",
	//"Content-Security-Policy": "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'",

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
