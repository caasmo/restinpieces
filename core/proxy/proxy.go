package proxy

import (
	"net/http"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

// represent the Proxy of an app handler
// mostly global conunters, metrics, and most important blocking, filtering 
// maybe can be part of app. app has a proxy, 
type Proxy struct {
	// TODO app http.Handler no Proxy needs all the services of app
	// app is also handler, the serverHttp method is to call its router
	app       *core.App
	ipBlocker FeatureBlocker
}

// Feature defines an interface for features that can be enabled or disabled.
type Feature interface {
	IsEnabled() bool
}

// Blocker defines an interface for checking if an IP address is blocked.
type Blocker interface {
	IsBlocked(ip string) bool
}

// FeatureBlocker combines the Feature and Blocker interfaces.
type FeatureBlocker interface {
	Feature
	Blocker
}

// NewProxy creates a new Proxy instance with the given app and configures its features.
func NewProxy(app *core.App) *Proxy {
	px := &Proxy{
		app: app,
		// config is no longer stored directly on Proxy
	}

	// Initialize the IP Blocker based on application configuration
	if app.Config().Proxy.BlockIp.Enabled {
		// Pass the application's cache to the BlockIp implementation
		px.ipBlocker = NewBlockIp(app.Cache())
	} else {
		px.ipBlocker = &DisabledBlock{}
	}

	return px
}

func (px *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if IP blocking is enabled first
	if px.ipBlocker.IsEnabled() {
		// Get client IP from request using app's method
		ip := px.app.GetClientIP(r)

		// Check if the IP is blocked using the configured blocker
		if px.ipBlocker.IsBlocked(ip) {
			// TODO: Implement actual blocking response (e.g., http.StatusForbidden)
			px.app.Logger().Warn("blocked request from IP", "ip", ip)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}


	px.app.Router().ServeHTTP(w, r)
}

