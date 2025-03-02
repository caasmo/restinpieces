package httprouter

import (
	"context"
	"github.com/caasmo/restinpieces/router"
	jshttprouter "github.com/julienschmidt/httprouter"
	"net/http"
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

func (r *Router) Params(req *http.Request) router.Params {
	pms, _ := req.Context().Value(jshttprouter.ParamsKey).(jshttprouter.Params)
	var params router.Params
	for _, v := range pms {
		params = append(params, router.Param{Key: v.Key, Value: v.Value})
	}
	return params
}

func New() router.Router {
	return &Router{rt: jshttprouter.New()}
}

