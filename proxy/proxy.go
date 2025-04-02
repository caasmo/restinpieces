package proxy

import (
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/router"
)

type Proxy struct {
	r      router.Router
	config *config.Config
}

// NewProxy creates a new Proxy instance with the given router and config
func NewProxy(r router.Router, cfg *config.Config) *Proxy {
	return &Proxy{
		r:      r,
		config: cfg,
	}
}

func (px *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	domain := getDomain(r.Host)
	path := r.URL.Path
	
	if allowed := px.isPathAllowedForDomain(domain, path); !allowed {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	
	px.r.ServeHTTP(w, r)
}

// getDomain extracts the main domain from host
func getDomain(host string) string {
	parts := strings.Split(host, ":")
	return parts[0] // Remove port if present
}

func (px *Proxy) isPathAllowedForDomain(domain, path string) bool {
	// Check if domain exists in OAuth2 providers
	if _, exists := px.config.OAuth2Providers[domain]; exists {
		return true
	}

	// Check against endpoints configuration
	for _, endpoint := range []string{
		px.config.Endpoints.RefreshAuth,
		px.config.Endpoints.RequestEmailVerification,
		px.config.Endpoints.ConfirmEmailVerification,
		// Add other endpoints as needed
	} {
		if strings.HasPrefix(path, config.Endpoints{}.Path(endpoint)) {
			return true
		}
	}

	return false
}
