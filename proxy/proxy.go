package proxy

import (
	"github.com/caasmo/restinpieces/router"
)

type Proxy struct {
	r           router.Router
	domainRules map[string]map[string]bool // map[domain]map[path]allowed
}

// NewProxy creates a new Proxy instance with the given router and domain rules configuration
func NewProxy(r router.Router, domainRules map[string]map[string]bool) *Proxy {
	return &Proxy{
		r:           r,
		domainRules: domainRules,
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

func (m *MultiDomainMux) isPathAllowedForDomain(domain, path string) bool {
    rules, exists := m.domainRules[domain]
    if !exists {
        return false
    }
    
    // Check if path is allowed
    return rules[path]
}
