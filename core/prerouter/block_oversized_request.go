package prerouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

// BlockOversizedRequest guards against request flooding and resource exhaustion attacks
// by enforcing configurable size limits across all major request dimensions:
//
//   - URL path length
//   - Query string length
//   - Number of headers
//   - Individual header value length
//   - Request body size
//
// Checks are applied cheapest-first: the inexpensive string-length and count
// comparisons run before the lazy body limit (which only triggers on read).
// A request is rejected as soon as any single limit is exceeded, so the
// remaining checks are skipped entirely.
//
// All rejections return the appropriate HTTP status code:
//   - 414 URI Too Long        — URL path or query string exceeds limit
//   - 431 Request Header Fields Too Large — header count or a single header value exceeds limit
//   - 413 Content Too Large   — body exceeds limit (handled by http.MaxBytesReader)
type BlockOversizedRequest struct {
	app *core.App // Use App to access config
}

// NewBlockOversizedRequest creates a new BlockOversizedRequest middleware instance.
func NewBlockOversizedRequest(app *core.App) *BlockOversizedRequest {
	return &BlockOversizedRequest{
		app: app,
	}
}

// Execute wraps the next handler with multi-dimensional request size limiting logic.
//
// The check order is intentionally cheapest-first to fail fast with minimal work:
//
//  1. URL path length      — single len() call on a string already in memory
//  2. Query string length  — same cost as path check
//  3. Header count         — single len() call on the header map
//  4. Header value lengths — O(n) over header map entries; slightly more work but
//     still CPU-bound and done entirely in memory
//  5. Body limit           — delegated to http.MaxBytesReader, which is lazy:
//     it only fires when the handler (or a decoder downstream) actually reads
//     r.Body. This must be set before calling next.ServeHTTP so it is in place
//     for whoever reads the body first.
func (b *BlockOversizedRequest) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := b.app.Config().BlockOversizedRequest

		// Skip if middleware is not activated
		if !cfg.Activated {
			next.ServeHTTP(w, r)
			return
		}

		// Check if path is in excluded paths.
		// Excluded paths bypass all size checks in this middleware entirely.
		// Use this for endpoints that legitimately accept large payloads
		// (e.g. file upload routes).
		for _, path := range cfg.ExcludedPaths {
			if r.URL.Path == path {
				next.ServeHTTP(w, r)
				return
			}
		}

		// ----------------------------------------------------------------
		// CHECK 1: URL path length
		//
		// An unusually long URL path can be used to exhaust log buffers,
		// overflow fixed-size path-matching arrays in downstream routers,
		// or trigger worst-case behaviour in regex-based routing.
		//
		// r.URL.Path is already parsed and percent-decoded by the time it
		// reaches this middleware, so we measure the decoded length. This
		// is consistent with what downstream handlers receive.
		//
		// RFC 9110 does not define a maximum URL length, so we rely on a
		// configurable limit. A common safe default is 2048 bytes.
		//
		// 414 URI Too Long is the correct status for an oversized request
		// target (RFC 9110 §15.5.15).
		// ----------------------------------------------------------------
		if cfg.URLPathLimit > 0 && len(r.URL.Path) > cfg.URLPathLimit {
			w.WriteHeader(http.StatusRequestURITooLong)
			return
		}

		// ----------------------------------------------------------------
		// CHECK 2: Query string length
		//
		// Long query strings are a classic DoS and injection vector. Parsers
		// like url.ParseQuery allocate a new map entry for every key=value
		// pair, so an attacker can craft a query string that causes the
		// server to allocate significant memory just from parsing — before
		// any business logic runs.
		//
		// We check r.URL.RawQuery (the raw, still-encoded form) rather than
		// the decoded query string. This is intentional: the raw form is
		// available without any additional allocation, and an encoded value
		// is never shorter than its decoded counterpart (percent-encoding
		// expands characters to 3 bytes), so the raw length is a conservative
		// upper bound that's free to compute.
		//
		// 414 URI Too Long also covers an oversized query component
		// (RFC 9110 §15.5.15).
		// ----------------------------------------------------------------
		if cfg.QueryStringLimit > 0 && len(r.URL.RawQuery) > cfg.QueryStringLimit {
			w.WriteHeader(http.StatusRequestURITooLong)
			return
		}

		// ----------------------------------------------------------------
		// CHECK 3: Header count
		//
		// Some attack patterns send thousands of small, valid headers to
		// exhaust server memory. Go's net/http already applies MaxHeaderBytes
		// at the transport level (default 1 MB for the entire header block),
		// but that limit is cumulative across all header data and does not
		// bound the number of individual keys. A high count of short headers
		// can still slip through.
		//
		// Note: Go's http.Header stores multiple values per canonical key as
		// a []string slice, so len(r.Header) counts distinct canonical header
		// names, not total header lines. A single header name repeated N times
		// on the wire is stored as one key with N values. This check therefore
		// bounds the key fan-out, not the total wire line count. Adjust the
		// limit with that in mind.
		//
		// 431 Request Header Fields Too Large is the correct status
		// (RFC 6585 §5).
		// ----------------------------------------------------------------
		if cfg.HeaderCountLimit > 0 && len(r.Header) > cfg.HeaderCountLimit {
			w.WriteHeader(http.StatusRequestHeaderFieldsTooLarge)
			return
		}

		// ----------------------------------------------------------------
		// CHECK 4: Individual header value length
		//
		// Oversized individual header values — particularly Cookie,
		// Authorization, or custom application headers — can crash log
		// processors, overwhelm fixed-size buffers in WAFs or proxies, or
		// carry encoded payloads that exploit downstream parsers.
		//
		// We iterate over every (canonical key → []string values) pair and
		// check the byte length of each individual value string. Multiple
		// values for the same key are checked independently; the limit
		// applies per value, not per key.
		//
		// This is O(total header bytes) in the worst case, but header blocks
		// are already bounded by Go's MaxHeaderBytes, so the work is capped.
		//
		// 431 Request Header Fields Too Large (RFC 6585 §5).
		// ----------------------------------------------------------------
		if cfg.HeaderValueLimit > 0 {
			for _, values := range r.Header {
				for _, v := range values {
					if len(v) > cfg.HeaderValueLimit {
						w.WriteHeader(http.StatusRequestHeaderFieldsTooLarge)
						return
					}
				}
			}
		}

		// ----------------------------------------------------------------
		// CHECK 5: Request body size
		//
		// http.MaxBytesReader handles various cases:
		// 1. If Content-Length header exists and is > limitBytes, it immediately rejects.
		// 2. For chunked encoding, or if Content-Length is within limits (or absent),
		//    it wraps r.Body. If reading from r.Body exceeds limitBytes, the Read
		//    operation will fail, and MaxBytesReader sends a 413 response.
		//
		// It's important that MaxBytesReader is set *before* the handler tries to read the body.
		// The server usually makes sure r.Body is non-nil (e.g., http.NoBody for GET).
		// MaxBytesReader handles http.NoBody gracefully.
		//
		// Unlike the checks above, this one is lazy: the 413 is only sent
		// when someone actually reads past the limit. Setting it here ensures
		// it is in place for whoever reads first — the next handler, a JSON
		// decoder, a form parser, etc.
		// ----------------------------------------------------------------
		if cfg.BodyLimit > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, cfg.BodyLimit)
		}

		// Call the next handler in the chain.
		// If the next handler (or any subsequent code) tries to read r.Body
		// and exceeds the limit, the Read will fail, and MaxBytesReader
		// will have already sent the 413 response.
		next.ServeHTTP(w, r)
	})
}
