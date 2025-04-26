package router

import (
	"net/http"
)

// Chains represents a collection of route paths mapped to their handler Chains.
type Chains map[string]*Chain

type Router interface {
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
	ServeHTTP(http.ResponseWriter, *http.Request)
	Param(*http.Request, string) string
	Register(Chains)
}
