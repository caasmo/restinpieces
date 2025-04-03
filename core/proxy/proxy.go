package proxy

import (
	"net/http"
	"strings"
	"github.com/caasmo/restinpieces/core"
)

type Proxy struct {
	app *core.App
}

// NewProxy creates a new Proxy instance with the given app
func NewProxy(app *core.App) *Proxy {
	return &Proxy{
		app: app,
	}
}

func (px *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get client IP from request
	ip := r.RemoteAddr
	if colonIndex := strings.LastIndex(ip, ":"); colonIndex != -1 {
		ip = ip[:colonIndex] // Remove port if present
	}

	// Block IP if it's not already blocked
	if !px.IsBlocked(ip) {
		if err := px.BlockIP(ip); err != nil {
			px.app.Logger().Error("failed to block IP", "ip", ip, "err", err)
		}
	}

	px.app.Router().ServeHTTP(w, r)
}

// getDomain extracts the main domain from host
func getDomain(host string) string {
	parts := strings.Split(host, ":")
	return parts[0] // Remove port if present
}

