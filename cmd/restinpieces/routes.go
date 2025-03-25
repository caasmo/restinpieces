package main

import (
	"github.com/caasmo/restinpieces/core"
	r "github.com/caasmo/restinpieces/router"
	"net/http"

	// custom handlers and middleware
	"github.com/caasmo/restinpieces/custom"
)

func route(cfg *config.Config, ap *app.App, cAp *custom.App) {
	// Serve static files from configured public directory
	fs := http.FileServer(http.Dir(cfg.PublicDir))
	ap.Router().Handle("/", fs)
	//ap.Router().Handle("/assets/", http.StripPrefix("/assets/", fs))

	commonNewMiddleware := []func(http.Handler) http.Handler{ap.Logger}

	// API routes with explicit /api prefix
	ap.Router().Register(
		r.NewRoute("POST /api/auth-refresh").WithHandlerFunc(ap.RefreshAuthHandler).WithMiddleware(ap.JwtValidate),
		r.NewRoute("POST /api/auth-with-password").WithHandlerFunc(ap.AuthWithPasswordHandler),
		r.NewRoute("POST /api/auth-with-oauth2").WithHandlerFunc(ap.AuthWithOAuth2Handler),
		r.NewRoute("POST /api/request-verification").WithHandlerFunc(ap.RequestVerificationHandler),
		r.NewRoute("POST /api/register-with-password").WithHandlerFunc(ap.RegisterWithPasswordHandler),
		r.NewRoute("GET /api/list-oauth2-providers").WithHandlerFunc(ap.ListOAuth2ProvidersHandler).WithMiddlewareChain(commonNewMiddleware),
		r.NewRoute("POST /api/confirm-verification").WithHandlerFunc(ap.ConfirmVerificationHandler),

		//custom routes example: mixing core middleware and custom handler
		r.NewRoute("GET /custom").WithHandlerFunc(cAp.Index).WithMiddleware(ap.JwtValidate),
	)

	//ap.Router().Handle("/api/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	//ap.Router().Handle("GET /api", authMiddleware.ThenFunc(ap.Index))
	//ap.Router().Handle("/api/example/sqlite/read/randompk", http.HandlerFunc(ap.ExampleSqliteReadRandom))
	//ap.Router().Handle("/api/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	//ap.Router().Handle("/api/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	//ap.Router().Handle("/api/benchmark/sqlite/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	//ap.Router().Handle("GET /api/benchmark/sqlite/pool/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	//ap.Router().Handle("/api/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	//ap.Router().Handle("/api/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
}
