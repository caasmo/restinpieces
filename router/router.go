package router

import (
	"net/http"
)

type Router interface {
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
	ServeHTTP(http.ResponseWriter, *http.Request)
	Param(*http.Request, string) string
}

// Route builder for creating handler chains with middleware
type Route struct {
	endpoint    string
	handler     http.Handler
	middlewares []func(http.Handler) http.Handler
}

// NewRoute creates a new Route instance with initialized middlewares slice
// endpoint parameter is required - provides HTTP method and path pattern
func NewRoute(endpoint string) *Route {
	return &Route{
		endpoint:    endpoint,
		middlewares: make([]func(http.Handler) http.Handler, 0),
	}
}

// WithHandler sets the final handler for the route
func (r *Route) WithHandler(h http.Handler) *Route {
	r.handler = h
	return r
}

// WithHandlerFunc sets the final handler function for the route
func (r *Route) WithHandlerFunc(h http.HandlerFunc) *Route {
	return r.WithHandler(h)
}

// WithMiddleware adds one or more middlewares to the chain (prepended in reverse order)
func (r *Route) WithMiddleware(middlewares ...func(http.Handler) http.Handler) *Route {
	// Prepend in reverse order to maintain proper wrapping order
	for i := len(middlewares) - 1; i >= 0; i-- {
		r.middlewares = append([]func(http.Handler) http.Handler{middlewares[i]}, r.middlewares...)
	}
	return r
}

// WithMiddlewareChain prepends a chain of middlewares (added in given order)
func (r *Route) WithMiddlewareChain(middlewares []func(http.Handler) http.Handler) *Route {
	return r.WithMiddleware(middlewares...)
}

// Handler returns the final handler with all middlewares applied
func (r *Route) Handler() http.Handler {
	handler := r.handler
	// Apply middlewares in reverse registration order (outermost first)
	for _, mw := range r.middlewares {
		handler = mw(handler)
	}
	return handler
}
