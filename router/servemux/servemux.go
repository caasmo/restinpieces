package servemux

import (
	"net/http"
	router "github.com/caasmo/restinpieces/router"
)

// ServeMuxRouter implements router.Router using net/http ServeMux
type ServeMuxRouter struct {
	*http.ServeMux
}

func (s *ServeMuxRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.ServeMux.ServeHTTP(w, r)
}

func (s *ServeMuxRouter) Handle(path string, handler http.Handler) {
	s.ServeMux.Handle(path, handler)
}

func (s *ServeMuxRouter) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) {
	s.ServeMux.HandleFunc(path, handler)
}

func (s *ServeMuxRouter) Param(req *http.Request, key string) string {
	// Uses Go 1.22's PathValue which handles named parameters
	return req.PathValue(key)
}

func New() router.Router {
	return &ServeMuxRouter{ServeMux: http.NewServeMux()}
}
