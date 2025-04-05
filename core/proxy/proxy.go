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
	app             *core.App
	ipBlocker       FeatureBlocker
	mimetypeBlocker FeatureBlocker // Change type to interface
}

// Feature defines an interface for features that can be enabled or disabled.
type Feature interface {
	IsEnabled() bool
}

// Blocker defines an interface for checking if an IP address is blocked, blocking it,
// and processing requests to identify IPs for blocking.
type Blocker interface {
	IsBlocked(ip string) bool
	Block(ip string) error      // Adds the IP to the block list
	Process(ip string) error // Processes the IP (e.g., via sketch), returns error on failure
}

// FeatureBlocker combines the Feature and Blocker interfaces.
type FeatureBlocker interface {
	Feature
	Blocker
}

// DisabledBlock implements the FeatureBlocker interface but always returns false,
// effectively disabling the blocking feature.
type DisabledBlock struct{}

// IsEnabled always returns false, indicating the feature is disabled.
func (d *DisabledBlock) IsEnabled() bool {
	return false
}

// Block for DisabledBlock does nothing and returns nil.
func (d *DisabledBlock) Block(ip string) error {
	return nil // Blocking is disabled
}

// Process for DisabledBlock does nothing and returns nil.
func (d *DisabledBlock) Process(ip string) error {
	return nil // Blocking is disabled
}

// IsBlocked always returns false, indicating no IP is ever blocked.
func (d *DisabledBlock) IsBlocked(ip string) bool {
	return false
}

// NewProxy creates a new Proxy instance with the given app and configures its features.
func NewProxy(app *core.App) *Proxy {
	px := &Proxy{
		app: app,
		// config is no longer stored directly on Proxy
		// mimetypeBlocker initialized below based on config
	}

	// Initialize Mimetype Blocker based on configuration
	if app.Config().Proxy.Mimetype.Enabled {
		px.mimetypeBlocker = NewBlockMimetype(app)
	} else {
		px.mimetypeBlocker = &DisabledBlock{}
	}

	// Call the method to set up the ipBlocker based on config
	// TODO
	px.UpdateByConfig()

	return px
}

// UpdateByConfig configures the Proxy's features, like the IP blocker,
// based on the current application configuration.
func (px *Proxy) UpdateByConfig() {
	// Initialize the IP Blocker based on application configuration
	if px.app.Config().Proxy.BlockIp.Enabled {
		// Pass the application's cache and logger to the BlockIp implementation
		px.ipBlocker = NewBlockIp(px.app.Cache(), px.app.Logger())
	} else {
		px.ipBlocker = &DisabledBlock{}
	}
}

func (px *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if Mimetype blocking is enabled
	if px.mimetypeBlocker.IsEnabled() {
		contentType := r.Header.Get("Content-Type")
		if px.mimetypeBlocker.IsBlocked(contentType) {
			px.app.Logger().Error("enabled mime")

			// Return 415 Unsupported Media Type
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return // Stop processing
		}
	}

	// Check if IP blocking is enabled first
	if px.ipBlocker.IsEnabled() {
		// Get client IP from request using app's method
		ip := px.app.GetClientIP(r)

		// Check if the IP is already blocked (cache check)
		if px.ipBlocker.IsBlocked(ip) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		} else {
			// Process the IP (e.g., add to sketch). Log any processing errors.
			if err := px.ipBlocker.Process(ip); err != nil {
				// Log the error but typically continue processing the request,
				// as failure here might just mean the sketch update failed.
				px.app.Logger().Error("Error processing IP in blocker", "ip", ip, "error", err)
			}
		}
	}


	px.app.Router().ServeHTTP(w, r)
}

