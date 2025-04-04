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
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Process the request through the TopK sketch
		// This requires BlockIp to expose its sketch or a method to process ticks.
		// Let's assume BlockIp has a method ProcessRequest(ip string) []string
		// that internally calls sketch.processTick and returns IPs to block.
		// We need to add this method to BlockIp.

		// --- Add ProcessRequest method to BlockIp ---
		// In core/proxy/Block.go:
		// func (b *BlockIp) ProcessRequest(ip string) []string {
		// 	 return b.sketch.processTick(ip)
		// }
		// ---

		// Check if the sketch identified this or other IPs to block
		// Need to cast ipBlocker to *BlockIp to access ProcessRequest, or add ProcessRequest to the interface.
		// Adding to interface is cleaner.
		// --- Add ProcessRequest to Blocker interface ---
		// type Blocker interface {
		// 	 IsBlocked(ip string) bool
		// 	 Block(ip string) error
		// 	 ProcessRequest(ip string) []string // Returns IPs identified for blocking by the sketch
		// }
		// --- Implement in DisabledBlock ---
		// func (d *DisabledBlock) ProcessRequest(ip string) []string { return nil }
		// ---

		// Let's assume the interface and methods are added for now.
		if blockIPs := px.ipBlocker.ProcessRequest(ip); blockIPs != nil {
			for _, blockIP := range blockIPs {
				// Block the IP using the blocker's Block method
				if err := px.ipBlocker.Block(blockIP); err != nil {
					px.app.Logger().Error("failed to block IP identified by sketch", "ip", blockIP, "error", err)
					// Decide if we should continue or return an error here.
					// For now, log and continue; the current request might still be allowed if it wasn't the one blocked.
				} else {
					// If the *current* request's IP was just blocked, return Forbidden immediately.
					if blockIP == ip {
						px.app.Logger().Warn("blocked request from IP identified by sketch", "ip", ip)
						http.Error(w, "Forbidden", http.StatusForbidden)
						return
					}
				}
			}
		}
	}

	// If blocking is disabled or the IP is not blocked, proceed to the app router
	px.app.Router().ServeHTTP(w, r)
}

