package proxy

import (
	"github.com/caasmo/restinpieces/router"
)

type Proxy struct {
// other names controller, GateKeeper, 

	r router.Router 
    //domainRules map[string]map[string]bool // map[domain]map[path]allowed
}

func (px *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    //domain := getDomain(r.Host)
    //path := r.URL.Path
    
    // Check if this path is allowed for this domain
    //if allowed := m.isPathAllowedForDomain(domain, path); !allowed {
    //    http.Error(w, "Not found", http.StatusNotFound)
    //    return
    //}
    
    // Pass to standard mux
    px.r.ServeHTTP(w, r)
}

func (m *MultiDomainMux) isPathAllowedForDomain(domain, path string) bool {
    rules, exists := m.domainRules[domain]
    if !exists {
        return false
    }
    
    // Check if path is allowed
    return rules[path]
}
