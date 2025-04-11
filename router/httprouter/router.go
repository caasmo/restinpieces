package httprouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/core" // Added import
	"github.com/caasmo/restinpieces/router"
	jshttprouter "github.com/julienschmidt/httprouter"
)

// Implementation of the router interface
type Router struct {
	rt *jshttprouter.Router
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.rt.ServeHTTP(w, req)
}

// splitMethodPath extracts HTTP method from path pattern of form "METHOD /path"
// Returns method and cleaned path
// TODO code review and refactor, seems ugly
func splitMethodPath(fullPath string) (string, string) {
	if len(fullPath) == 0 || fullPath[0] == '/' {
		return "GET", fullPath // Default to GET if no method specified
	}

	// Split into method and path components
	for i, c := range fullPath {
		if c == ' ' || c == '/' {
			if i == 0 {
				return "GET", fullPath // Invalid empty method, default to GET
			}
			method := fullPath[:i]
			path := fullPath[i+1:]
			return method, "/" + path
		}
	}

	return "GET", fullPath // No separator found, treat entire string as path
}

func (r *Router) Handle(path string, handler http.Handler) {
	method, path := splitMethodPath(path)
	r.rt.Handler(method, path, handler)
}

func (r *Router) HandleFunc(path string, handleFunc func(http.ResponseWriter, *http.Request)) {
	method, path := splitMethodPath(path)
	r.rt.HandlerFunc(method, path, handleFunc)
}

func (r *Router) Param(req *http.Request, key string) string {
	pms, _ := req.Context().Value(jshttprouter.ParamsKey).(jshttprouter.Params)
	for _, p := range pms {
		if p.Key == key {
			return p.Value
		}
	}
	return ""
}

// Register registers multiple handler chains provided in a map.
// It delegates to the Handle method for each pattern and chain.
func (r *Router) Register(chains map[string]*router.Chain) {
	for pattern, chain := range chains {
		// Call Handle, which internally calls splitMethodPath and rt.Handler
		// It also calls chain.Handler() to get the final http.Handler
		r.Handle(pattern, chain.Handler())
	}
}

func New() router.Router {
	return &Router{rt: jshttprouter.New()}
}
