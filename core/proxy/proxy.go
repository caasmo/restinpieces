package proxy

import (
	"net/http"

	//"github.com/caasmo/restinpieces/config"
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

// Blocker defines an interface for checking if an IP address is blocked and blocking it.
type Blocker interface {
	IsBlocked(ip string) bool
	Block(ip string) error // Adds the IP to the block list
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
		// Pass the application's cache and logger to the BlockIp implementation
		px.ipBlocker = NewBlockIp(app.Cache(), app.Logger())
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

		// Check if the IP is already blocked (cache check)
		if px.ipBlocker.IsBlocked(ip) {
			px.app.Logger().Warn("blocked request from already blocked IP", "ip", ip)
			// TODO
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	} // <-- Add missing closing brace for the 'if px.ipBlocker.IsEnabled()' block


	px.app.Router().ServeHTTP(w, r)
}

