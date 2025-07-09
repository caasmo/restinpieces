package prerouter

import (
	"net"
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/core"
)

// BlockHost blocks requests based on the Host header.
type BlockHost struct {
	app *core.App // Use App to access config
}

// NewBlockHost creates a new Host header blocking middleware instance.
func NewBlockHost(app *core.App) *BlockHost {
	return &BlockHost{
		app: app,
	}
}

// Execute wraps the next handler with Host header blocking logic.
func (b *BlockHost) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := b.app.Config().BlockHost

		// If not activated or if AllowedHosts is empty, all hosts are allowed.
		if !cfg.Activated || len(cfg.AllowedHosts) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		// Use net.SplitHostPort to reliably separate host and port
		requestHost, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			// If SplitHostPort fails, it might be because there's no port.
			// In that case, r.Host is just the host, so we use it directly.
			// This also covers cases like IPv6 addresses without brackets.
			requestHost = r.Host
		}

		matched := false
		for _, allowedHost := range cfg.AllowedHosts {
			// Check for wildcard match (e.g., *.example.com)
			if strings.HasPrefix(allowedHost, "*.") {
				suffix := allowedHost[1:] // e.g., ".example.com"
				// The request host must end with the suffix, but must not be the bare domain itself.
				// e.g., "sub.example.com" should match, but "example.com" should not.
				if strings.HasSuffix(requestHost, suffix) && requestHost != suffix[1:] {
					matched = true
					break
				}
			} else {
				// Check for exact match
				if requestHost == allowedHost {
					matched = true
					break
				}
			}
		}

		if !matched {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}