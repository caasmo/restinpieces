package main

import (
	"github.com/caasmo/restinpieces/app"
	"github.com/justinas/alice"
	"net/http"

	// custom handlers and middleware
	"github.com/caasmo/restinpieces/custom"
)

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
// a func func (a *App) Index(w http.ResponseWriter, r *http.Request) is casted in the signature
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

// TODO encapsulate alice
// provide methods for
// - middlewareeChain 
// - Attach the middlwrare chain to some handler 
// - PostHandlers, run always, do not modify response.
func route(ap *app.App, cAp *custom.App) {
	// Serve static files from public directory
	fs := http.FileServer(http.Dir("public"))
	ap.Router().Handle("/", fs) 
	//ap.Router().Handle("/assets/", http.StripPrefix("/assets/", fs))

    // 
	commonMiddleware := alice.New(ap.SecurityHeadersMiddleware, ap.Logger)
	authMiddleware := alice.New(ap.JwtValidate)

	authNewMiddleware := []func(http.Handler) http.Handler{ap.JwtValidate}

	// Example route using Route builder with JWT validation
    //r := NewRoute("GET /api/route").WithHandlerFunc(ap.Index).WithMiddleware(ap.JwtValidate)
    r := NewRoute("GET /api/route").WithHandlerFunc(ap.Index).WithMiddlewareChain(authNewMiddleware)
    ap.Router().Handle(r.endpoint, r.Handler())
    r = NewRoute("GET /api/route2").WithHandlerFunc(ap.Index)
    ap.Router().Handle(r.endpoint, r.Handler())
    //ap.Router().Register(
    //    NewRoute("GET /api/route2").WithHandlerFunc(ap.Index)
    //    NewRoute("GET /api/route2").WithHandlerFunc(ap.Index)
    //    NewRoute("GET /api/route").WithHandlerFunc(ap.Index).WithMiddleware(ap.JwtValidate)
    //)

	// API routes with explicit /api prefix
	ap.Router().Handle("POST /api/auth-refresh", authMiddleware.ThenFunc(ap.RefreshAuthHandler))
	ap.Router().Handle("POST /api/auth-with-password", http.HandlerFunc(ap.AuthWithPasswordHandler))
	ap.Router().Handle("POST /api/auth-with-oauth2", http.HandlerFunc(ap.AuthWithOAuth2Handler))
	ap.Router().Handle("POST /api/request-verification", http.HandlerFunc(ap.RequestVerificationHandler))
	ap.Router().Handle("POST /api/register-with-password", http.HandlerFunc(ap.RegisterWithPasswordHandler))
	ap.Router().Handle("GET /api/list-oauth2-providers", commonMiddleware.ThenFunc(ap.ListOAuth2ProvidersHandler))

    //
    // custom route, example uses core middleware, showing how mix core and custom
    //
	ap.Router().Handle("GET /custom", authMiddleware.ThenFunc(cAp.Index))

	ap.Router().Handle("/api/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	ap.Router().Handle("GET /api", authMiddleware.ThenFunc(ap.Index))
	ap.Router().Handle("/api/example/sqlite/read/randompk", http.HandlerFunc(ap.ExampleSqliteReadRandom))
	ap.Router().Handle("/api/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	ap.Router().Handle("/api/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	ap.Router().Handle("/api/benchmark/sqlite/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	ap.Router().Handle("GET /api/benchmark/sqlite/pool/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	ap.Router().Handle("/api/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	ap.Router().Handle("/api/teas/:id", commonMiddleware.ThenFunc(ap.Tea))

}
