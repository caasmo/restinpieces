package main

import (
	"github.com/caasmo/restinpieces/app"
	"github.com/justinas/alice"
	"net/http"
)

func route(ap *app.App) {
	// Serve static files from public directory
	fs := http.FileServer(http.Dir("public"))
	ap.Router().Handle("/", fs)
	ap.Router().Handle("/assets/", http.StripPrefix("/assets/", fs))

	commonMiddleware := alice.New(ap.SecurityHeadersMiddleware, ap.Logger)
	authMiddleware := alice.New(ap.JwtValidate)
	// API routes with /api prefix - we add prefix directly since our router
	// interface doesn't support PathPrefix/Subrouter functionality
	apiRouter := ap.Router().PathPrefix("/api").Subrouter()
	apiRouter.Handle("POST /auth-refresh", authMiddleware.ThenFunc(ap.RefreshAuthHandler))
	apiRouter.Handle("POST /auth-with-password", http.HandlerFunc(ap.AuthWithPasswordHandler))
	apiRouter.Handle("POST /auth-with-oauth2", http.HandlerFunc(ap.AuthWithOAuth2Handler))
	apiRouter.Handle("POST /request-verification", http.HandlerFunc(ap.RequestVerificationHandler))
	apiRouter.Handle("POST /register", http.HandlerFunc(ap.RegisterHandler))
	apiRouter.Handle("GET /oauth2-providers", commonMiddleware.ThenFunc(ap.OAuth2ProvidersHandler))

	apiRouter.Handle("/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	apiRouter.Handle("", authMiddleware.ThenFunc(ap.Index)) // /api
	apiRouter.Handle("/example/sqlite/read/randompk", http.HandlerFunc(ap.ExampleSqliteReadRandom))
	apiRouter.Handle("/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	apiRouter.Handle("/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	apiRouter.Handle("/benchmark/sqlite/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	apiRouter.Handle("GET /benchmark/sqlite/pool/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	apiRouter.Handle("/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	apiRouter.Handle("/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
}
