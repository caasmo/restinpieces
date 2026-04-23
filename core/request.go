package core

import (
	"net"
	"net/http"
	"strings"
)

const (
	MimeTypeJSON           = "application/json"
	MimeTypeHTML           = "text/html"
	MimeTypeJavaScript     = "application/javascript"
	MimeTypeJavaScriptText = "text/javascript"
)
// GetClientIP extracts the client IP address from the request, handling proxies via configured header
func (a *App) GetClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Handle error potentially, or use RemoteAddr directly if no port
		ip = r.RemoteAddr
	}

	cfg := a.Config() // Get the current config
	if cfg.Server.ClientIpProxyHeader != "" {
		if forwarded := r.Header.Get(cfg.Server.ClientIpProxyHeader); forwarded != "" {
			// Use the first IP in the list if header contains multiple
			parts := strings.Split(forwarded, ",")
			ip = strings.TrimSpace(parts[0])
		}
	}
	return ip
}
