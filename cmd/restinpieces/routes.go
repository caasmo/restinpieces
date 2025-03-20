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
	handler     http.HandlerFunc
	middlewares []func(http.Handler) http.Handler
}

// WithEndpoint sets the HTTP method and path pattern for the route
func (r *Route) WithEndpoint(pattern string) *Route {
	r.endpoint = pattern
	return r
}

// WithHandler sets the final handler function for the route
func (r *Route) WithHandler(h http.HandlerFunc) *Route {
	r.handler = h
	return r
}

// WithMiddleware adds one or more middlewares to the chain (appended in reverse order)
func (r *Route) WithMiddleware(middlewares ...func(http.Handler) http.Handler) *Route {
	// Reverse to maintain proper wrapping order
	for i := len(middlewares) - 1; i >= 0; i-- {
		r.middlewares = append(r.middlewares, middlewares[i])
	}
	return r
}

// WithMiddlewareChain prepends a chain of middlewares (added in given order)
func (r *Route) WithMiddlewareChain(middlewares []func(http.Handler) http.Handler) *Route {
	// Prepend to existing middlewares to maintain chain order
	r.middlewares = append(middlewares, r.middlewares...)
	return r
}

// Apply registers the route with the router using the built middleware chain
func (r *Route) Apply(ap *app.App) {
	var handler http.Handler = http.HandlerFunc(r.handler)
	// Apply middlewares in reverse registration order (outermost first)
	for _, mw := range r.middlewares {
		handler = mw(handler)
	}
	ap.Router().Handle(r.endpoint, handler)
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

    //
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
