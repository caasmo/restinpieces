package proxy

import (
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/router"
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
	//domain := getDomain(r.Host)
	//path := r.URL.Path
	//
	//if allowed := px.isPathAllowedForDomain(domain, path); !allowed {
	//	http.Error(w, "Not found", http.StatusNotFound)
	//	return
	//}

	px.app.Router().ServeHTTP(w, r)
}

// getDomain extracts the main domain from host
func getDomain(host string) string {
	parts := strings.Split(host, ":")
	return parts[0] // Remove port if present
}

func (px *Proxy) isPathAllowedForDomain(domain, path string) bool {
	// Check if domain exists in OAuth2 providers
	if _, exists := px.app.Config().OAuth2Providers[domain]; exists {
		return true
	}

	// Check against endpoints configuration
	for _, endpoint := range []string{
		px.app.Config().Endpoints.RefreshAuth,
		px.app.Config().Endpoints.RequestEmailVerification,
		px.app.Config().Endpoints.ConfirmEmailVerification,
		// Add other endpoints as needed
	} {
		if strings.HasPrefix(path, config.Endpoints{}.Path(endpoint)) {
			return true
		}
	}

	return false
}
