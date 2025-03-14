package main

import (
	"github.com/caasmo/restinpieces/app"
	"github.com/justinas/alice"
	"net/http"
)

func route(ap *app.App) {
	// Serve static files from public directory
	fs := http.FileServer(http.Dir("public"))
	ap.Router().Handle("/assets/", http.StripPrefix("/assets/", fs))

	commonMiddleware := alice.New(ap.SecurityHeadersMiddleware, ap.Logger)
	authMiddleware := alice.New(ap.JwtValidate)

	// API routes with explicit /api prefix
	ap.Router().Handle("POST /api/auth-refresh", authMiddleware.ThenFunc(ap.RefreshAuthHandler))
	ap.Router().Handle("POST /api/auth-with-password", http.HandlerFunc(ap.AuthWithPasswordHandler))
	ap.Router().Handle("POST /api/auth-with-oauth2", http.HandlerFunc(ap.AuthWithOAuth2Handler))
	ap.Router().Handle("POST /api/request-verification", http.HandlerFunc(ap.RequestVerificationHandler))
	ap.Router().Handle("POST /api/register", http.HandlerFunc(ap.RegisterHandler))
	ap.Router().Handle("GET /api/oauth2-providers", commonMiddleware.ThenFunc(ap.OAuth2ProvidersHandler))

	ap.Router().Handle("/api/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	ap.Router().Handle("/api", authMiddleware.ThenFunc(ap.Index))
	ap.Router().Handle("/api/example/sqlite/read/randompk", http.HandlerFunc(ap.ExampleSqliteReadRandom))
	ap.Router().Handle("/api/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	ap.Router().Handle("/api/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	ap.Router().Handle("/api/benchmark/sqlite/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	ap.Router().Handle("GET /api/benchmark/sqlite/pool/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	ap.Router().Handle("/api/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	ap.Router().Handle("/api/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
}
