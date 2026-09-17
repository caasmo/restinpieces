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

		if !isHostHeaderAllowed(r.Host, cfg.AllowedHosts) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isHostHeaderAllowed reports whether the Host header value matches any of the
// allowed host entries.
func isHostHeaderAllowed(requestHost string, allowedHosts []string) bool {
	requestHost = normalizeHost(requestHost)

	for _, allowedHost := range allowedHosts {
		if strings.HasPrefix(allowedHost, "*") {
			domain := allowedHost[1:] // e.g., "example.com"
			if requestHost == domain || strings.HasSuffix(requestHost, "."+domain) {
				return true
			}

			continue
		}

		if requestHost == allowedHost {
			return true
		}
	}

	return false
}

// normalizeHost returns the form used to compare hostnames: lowercase, with
// the port and a trailing dot removed. net.SplitHostPort separates the port
// reliably; when it fails there is no port, and the host is used as it is.
// This also covers IPv6 addresses without brackets.
func normalizeHost(host string) string {
	hostWithoutPort, _, err := net.SplitHostPort(host)
	if err == nil {
		host = hostWithoutPort
	}

	host = strings.TrimSuffix(host, ".")
	host = strings.Trim(host, "[]")

	return strings.ToLower(host)
}
