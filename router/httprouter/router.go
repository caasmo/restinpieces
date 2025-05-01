package httprouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/router"
	"strings"

	"github.com/caasmo/restinpieces/router"
	jshttprouter "github.com/julienschmidt/httprouter"
)

type Router struct {
	rt *jshttprouter.Router
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.rt.ServeHTTP(w, req)
}

func (r *Router) Handle(pattern string, handler http.Handler) {
	method, path := splitMethodPathPattern(pattern)
	r.rt.Handler(method, path, handler)
}

func (r *Router) HandleFunc(pattern string, handleFunc func(http.ResponseWriter, *http.Request)) {
	method, path := splitMethodPathPattern(pattern)
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

func (r *Router) Register(chains router.Chains) {
	for pattern, chain := range chains {
		r.Handle(pattern, chain.Handler())
	}
}

// splitMethodPathPattern splits "METHOD /path" into method and path.
// It defaults to "GET" if the method is missing.
func splitMethodPathPattern(pattern string) (method, path string) {
	parts := strings.SplitN(pattern, " ", 2)
	if len(parts) == 1 {
		// No method specified, assume GET and the whole string is the path
		return http.MethodGet, parts[0]
	}
	// Standard "METHOD /path" format
	return parts[0], parts[1]
}

func New() router.Router {
	return &Router{rt: jshttprouter.New()}
}
