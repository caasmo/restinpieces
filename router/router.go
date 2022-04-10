package router

import (
    "net/http"
    "github.com/julienschmidt/httprouter"
)

type Router struct {
    *httprouter.Router
}

func (r *Router) Get(path string, handler http.Handler) {
    r.Handler("GET", path, handler)
}

func New() *Router {
    return &Router{httprouter.New()}
}
