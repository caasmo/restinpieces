package core

import (
	"net/http"
)

// TODO consiten name
var HeadersJson = map[string]string{

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

	// the main XSS-prevention benefits of CSP don't apply to JSON responses
	// because they aren't treated as active documents by the browser. However,
	// using Content-Security-Policy: default-src 'none'; frame-ancestors
	// 'none'; is not entirely meaningless. It provides valuable
	// anti-clickjacking protection (frame-ancestors) and reinforces the
	// non-document nature of the response (default-src). It's a low-cost
	// security hardening step.
	//
	// frame-ancestors 'none': This directive is still relevant. It prevents
	// any domain (including your own) from embedding the API endpoint URL in
	// an <iframe>, <frame>, <object>, or <embed>. This provides protection
	// against Clickjacking attacks where an attacker might try to trick a user
	// into interacting with your API endpoint indirectly via a framed page.
	// While less common for APIs than for interactive web pages, it's a valid
	// defense-in-depth measure. This is the modern replacement for
	// X-Frame-Options: DENY.
	//
	// default-src 'none': Setting this essentially acts as a strong assertion:
	// "This response should never be interpreted as an active document capable
	// of loading resources." While the Content-Type header already signals
	// this, adding CSP: default-src 'none' provides an extra layer should
	// there ever be a browser bug or unusual scenario where the content type
	// is misinterpreted. It hardens the endpoint.
	"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
}

// headerCacheStatic defines cache headers for immutable static assets (CSS, JS, images).
// Assumes filename-based versioning for cache busting.
var headersStatic = map[string]string{
	// - public: Allows caching by intermediate proxies and browsers.
	// - max-age=31536000: Cache for 1 year.
	// - immutable: Indicates the file content will never change. Browsers
	//              will not even attempt to revalidate it, providing maximum
	//              caching efficiency. Relies entirely on filename versioning.
	"Cache-Control": "public, max-age=31536000, immutable",

	// See descrption above
	"X-Content-Type-Options": "nosniff",

	// Adding CSP to individual static assets doesn't provide security benefits
}

// headerCacheStaticHtml defines cache headers for HTML entry point files.
var headersStaticHtml = map[string]string{
	// - public: Allows caching by intermediate proxies and browsers.
	// - no-cache: Requires the cache to revalidate with the origin server
	//             before using a cached response. Ensures the latest HTML
	//             (with potentially updated asset links) is served.
	"Cache-Control": "public, no-cache",

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

	// The Referrer-Policy HTTP header controls how much referrer information
	// browsers include when navigating from your website to another site.
	// Send full URL for same-origin requests, only origin for cross-origin at
	// same security level, nothing when security decreases.
	"Referrer-Policy": "strict-origin-when-cross-origin",

	// (X-XSS-Protection):
	// Modern browsers (post-2019 Chrome, Edge, etc.) ignore this header, favoring Content Security Policy (CSP)
	// this header is mostly a legacy tool
	//"X-XSS-Protection":           "1; mode=block",
}

// HeadersFavicon defines cache headers for favicon.ico.
// Favicons are often requested frequently and don't change often.
var HeadersFavicon = map[string]string{
	// - public: Allows caching by intermediate proxies and browsers.
	// - max-age=86400: Cache for 24 hours. Favicons can be cached longer
	//                  than HTML but shorter than immutable assets.
	"Cache-Control": "public, max-age=86400",
}

// setHeaders applies one or more sets of headers to the response writer.
// Headers from later maps will overwrite headers from earlier maps if keys conflict.
func setHeaders(w http.ResponseWriter, headers ...map[string]string) {
	for _, headerMap := range headers {
		for key, value := range headerMap {
			// Using Set() is slightly cleaner than direct map access and handles potential nil map internally.
			w.Header().Set(key, value)
		}
	}
}
