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
	config    *config.Config
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

// NewProxy creates a new Proxy instance with the given app and config, and configures its features.
func NewProxy(app *core.App, cfg *config.Config) *Proxy {
	px := &Proxy{
		app:    app,
		config: cfg,
	}

	// Initialize the IP Blocker based on configuration
	if cfg.Proxy.BlockIp.Enabled {
		px.ipBlocker = NewBlockIp(cfg) // Pass the config
	} else {
		px.ipBlocker = &DisabledBlock{}
	}

	return px
}

func (px *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Get client IP from request using app's method
	ip := px.app.GetClientIP(r)

	// Check if the IP is blocked using the configured blocker
	if px.ipBlocker.IsBlocked(ip) {
		// TODO: Implement actual blocking response (e.g., http.StatusForbidden)
		px.app.Logger().Warn("blocked request from IP", "ip", ip)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	px.app.Logger().Warn("in Proxy", "ip", ip)

	// // Example of how blocking might be triggered (moved from here)
	// // Block IP if it's not already blocked
	// if !px.IsBlocked(ip) {
	// 	if err := px.BlockIP(ip); err != nil {
	// 		px.app.Logger().Error("failed to block IP", "ip", ip, "err", err)
	// 	}
	// }

	px.app.Router().ServeHTTP(w, r)
}

// IsBlocked checks if an IP is blocked using the configured ipBlocker.
//func (px *Proxy) IsBlocked(ip string) bool {
//	return px.ipBlocker.IsBlocked(ip)
//}

// TODO: Decide if BlockIP should be part of the Proxy or the Blocker interface itself.
// If part of the Blocker, the implementation in BlockIp struct would need access
// to the cache, logger etc., likely via the App instance.
// For now, keeping the original methods_block.go logic accessible via Proxy.

// BlockIP attempts to block the given IP address.
// This might be called from specific handlers upon detecting abuse.
//func (px *Proxy) BlockIP(ip string) error {
//	// Check if blocking is enabled at all
//	if !px.ipBlocker.IsEnabled() {
//		return nil // Blocking is disabled, do nothing
//	}
//	// Delegate to the core app's blocking logic (which uses the cache)
//	// This assumes the core.App retains the BlockIP method.
//	// If BlockIP logic moves entirely into the BlockIp struct, this needs adjustment.
//	return px.app.BlockIP(ip)
//}

