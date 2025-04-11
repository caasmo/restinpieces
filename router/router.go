package router

import (
	"github.com/caasmo/restinpieces/core"
	"net/http"
)

type Router interface {
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
	ServeHTTP(http.ResponseWriter, *http.Request)
	Param(*http.Request, string) string
	Register(map[string] *core.Chain)
}
