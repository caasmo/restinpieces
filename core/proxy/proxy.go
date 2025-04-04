package proxy

import (
	"net/http"

	"sync/atomic" // Import atomic
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
	ipBlocker atomic.Pointer[FeatureBlocker] 
	//ipBlocker FeatureBlocker
}

// Feature defines an interface for features that can be enabled or disabled.
type Feature interface {
	IsEnabled() bool
}

// Blocker defines an interface for checking if an IP address is blocked, blocking it,
// and processing requests to identify IPs for blocking.
type Blocker interface {
	IsBlocked(ip string) bool
	Block(ip string) error           // Adds the IP to the block list
	Process(ip string) []string // Processes the IP (e.g., via sketch) and returns IPs to block
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

	// Call the method to set up the blocker based on config
	px.UpdateByConfig(app.Config())

	return px
}

// UpdateByConfig configures the Proxy's features, like the IP blocker,
// based on the current application configuration (dinamically)
func (px *Proxy) UpdateByConfig(cfg *config.Config) {
	// Initialize the IP Blocker based on application configuration
	var newBlocker FeatureBlocker
	if cfg.Proxy.BlockIp.Enabled {
		// Pass the application's cache and logger to the BlockIp implementation
		newBlocker = NewBlockIp(px.app.Cache(), px.app.Logger())
	} else {
		newBlocker = &DisabledBlock{}
	}

	px.ipBlocker.Store(&newBlocker)
}

func (px *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	blockerPtr := px.ipBlocker.Load()
	// Check if IP blocking is enabled first
	// Dereference the pointer to get the actual interface value (FeatureBlocker)
	blocker := *blockerPtr

	if blocker.IsEnabled() {
		// Get client IP from request using app's method
		ip := px.app.GetClientIP(r)

		// Check if the IP is already blocked (cache check)
		if blocker.IsBlocked(ip) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		} else {
			blocker.Process(ip)
		}
	} 


	px.app.Router().ServeHTTP(w, r)
}

