package main

import (
	"github.com/caasmo/restinpieces/app"
	"github.com/caasmo/restinpieces/router"
	"github.com/justinas/alice"
	"net/http"

	// custom handlers and middleware
	"github.com/caasmo/restinpieces/custom"
)


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
	commonNewMiddleware := []func(http.Handler) http.Handler{ap.SecurityHeadersMiddleware, ap.Logger}

	// Example route using Route builder with JWT validation
    //r := NewRoute("GET /api/route").WithHandlerFunc(ap.Index).WithMiddleware(ap.JwtValidate)
    r := router.NewRoute("GET /api/route").WithHandlerFunc(ap.Index).WithMiddlewareChain(authNewMiddleware)
    ap.Router().Handle(r.Endpoint, r.Handler())
    r = router.NewRoute("GET /api/route2").WithHandlerFunc(ap.Index)
    ap.Router().Handle(r.Endpoint, r.Handler())
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
	//ap.Router().Handle("GET /api/list-oauth2-providers", commonMiddleware.ThenFunc(ap.ListOAuth2ProvidersHandler))
    r = router.NewRoute("GET /api/list-oauth2-providers").WithHandlerFunc(ap.ListOAuth2ProvidersHandler).WithMiddlewareChain(commonNewMiddleware)
    ap.Router().Handle(r.Endpoint, r.Handler())

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
