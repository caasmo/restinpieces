package core

import (
	"net/http"
)

type Chain struct {
	//Endpoint    string // exported field
	handler     http.Handler
	middlewares []func(http.Handler) http.Handler
	observers   []http.Handler
}

// NewRoute creates a new Route instance with initialized middlewares slice
// endpoint parameter is required - provides HTTP method and path pattern
func NewChain(h http.Handler) *Chain {
	//if endpoint == "" {
	//	panic("route endpoint cannot be empty")
	//}
	return &Chain{
		middlewares: make([]func(http.Handler) http.Handler, 0),
	}
}

// WithHandler sets the final handler for the route
//func (r *Route) WithHandler(h http.Handler) *Route {
//	r.handler = h
//	return r
//}

// WithHandlerFunc sets the final handler function for the route
//func (r *Route) WithHandlerFunc(h http.HandlerFunc) *Route {
//	return r.WithHandler(h)
//}

// WithMiddleware adds one or more middlewares to the chain.
// Middlewares execute in the order they are defined, from left to right.
// For example:
//
//	.WithMiddleware(mw1, mw2, mw3)
//
// Will execute as:
// 1. mw1 (first middleware runs first)
// 2. mw2
// 3. mw3
// 4. Handler
//
// This follows the same semantics as popular middleware chaining packages like
// Alice (github.com/justinas/alice) where the first middleware in the chain
// is the outermost handler that runs first. This matches the natural reading
// order of the code and makes it easier to reason about middleware execution.
func (r *Chain) WithMiddleware(middlewares ...func(http.Handler) http.Handler) *Chain {
	for _, mw := range middlewares {
		r.middlewares = append([]func(http.Handler) http.Handler{mw}, r.middlewares...)
	}
	return r
}

// WithMiddlewareChain prepends a chain of middlewares (added in given order)
func (r *Chain) WithMiddlewareChain(middlewares []func(http.Handler) http.Handler) *Chain {
	return r.WithMiddleware(middlewares...)
}

// WithObservers adds handlers that run after the handler and middleware chain.
// Observers are typically used for logging, metrics collection, and other side effects.
// Note that observers will execute even if middleware returns early or stops processing.
// Observers should not write to the response as the main handler may have already sent headers.
// Use carefully as this could lead to unintended side effects when middleware fails.
func (r *Chain) WithObservers(observers ...http.Handler) *Chain {
	r.observers = append(r.observers, observers...)
	return r
}

// Handler returns the final handler with all middlewares and observers applied
func (r *Chain) Handler() http.Handler {
	if r.handler == nil {
		panic("handler cannot be nil")
	}
	handler := r.handler

	for _, mw := range r.middlewares {
		handler = mw(handler)
	}

	// If no observers, return the middleware-wrapped handler directly
	if len(r.observers) == 0 {
		return handler
	}

	// Wrap handler with observers if present
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Run the main handler chain
		handler.ServeHTTP(w, req)

		// Run all observers in order they were added
		for _, obs := range r.observers {
			obs.ServeHTTP(w, req)
		}
	})
}
