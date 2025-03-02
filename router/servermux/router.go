package servermux

import (
	"net/http"
	router "github.com/caasmo/restinpieces/router"
)

// ServerMuxRouter implements router.Router using net/http ServeMux
type ServerMuxRouter struct {
	*http.ServeMux
}

func (s *ServerMuxRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.ServeMux.ServeHTTP(w, r)
}

func (s *ServerMuxRouter) Handle(path string, handler http.Handler) {
	s.ServeMux.Handle(path, handler)
}

func (s *ServerMuxRouter) HandleFunc(path string, handler func(http.ResponseWriter, *http.Request)) {
	s.ServeMux.HandleFunc(path, handler)
}

func (s *ServerMuxRouter) Param(req *http.Request, key string) string {
	// Uses Go 1.22's PathValue which handles named parameters
	return req.PathValue(key)
}

func New() router.Router {
	return &ServerMuxRouter{ServeMux: http.NewServeMux()}
}
