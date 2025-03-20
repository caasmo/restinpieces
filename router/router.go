package router

import (
	"net/http"
)

type Router interface {
	Handle(string, http.Handler)
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
	ServeHTTP(http.ResponseWriter, *http.Request)
	Param(*http.Request, string) string
	Register(routes ...*Route)
}

// Route builder for creating handler chains with middleware
type Route struct {
	Endpoint    string // exported field
	handler     http.Handler
	middlewares []func(http.Handler) http.Handler
	observers   []http.Handler
}

// NewRoute creates a new Route instance with initialized middlewares slice
// endpoint parameter is required - provides HTTP method and path pattern
func NewRoute(endpoint string) *Route {
	return &Route{
		Endpoint:    endpoint, // update to use exported field
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

// Handler returns the final handler with all middlewares and observers applied
func (r *Route) Handler() http.Handler {
	handler := r.handler
	
	// Apply middlewares in reverse registration order (outermost first)
	for _, mw := range r.middlewares {
		handler = mw(handler)
	}
	
	// If no observers, return the middleware-wrapped handler directly
	if len(r.observers) == 0 {
		return handler
	}
	
	// Wrap handler with observers if present
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Run the main handler chain
		handler.ServeHTTP(w, r)
		
		// Run all observers in order they were added
		for _, obs := range r.observers {
			obs.ServeHTTP(w, r)
		}
	})
}

// WithObservers adds handlers that run after the main handler
func (r *Route) WithObservers(observers ...http.Handler) *Route {
	r.observers = append(r.observers, observers...)
	return r
}
