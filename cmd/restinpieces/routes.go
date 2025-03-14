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
	ap.Router().Handle("POST /auth-refresh", authMiddleware.ThenFunc(ap.RefreshAuthHandler))
	ap.Router().Handle("POST /auth-with-password", http.HandlerFunc(ap.AuthWithPasswordHandler))
	ap.Router().Handle("POST /auth-with-oauth2", http.HandlerFunc(ap.AuthWithOAuth2Handler))
	ap.Router().Handle("POST /request-verification", http.HandlerFunc(ap.RequestVerificationHandler))
	ap.Router().Handle("POST /register", http.HandlerFunc(ap.RegisterHandler))

	ap.Router().Handle("/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	ap.Router().Handle("/api", authMiddleware.ThenFunc(ap.Index))
	ap.Router().Handle("/example/sqlite/read/randompk", http.HandlerFunc(ap.ExampleSqliteReadRandom))
	ap.Router().Handle("/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	//router.Handle("/example/ristretto/writeread/:value", http.HandlerFunc(ap.ExampleRistrettoWriteRead))
	ap.Router().Handle("/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	//ap.Router().Handle("/benchmark/sqlite/ratio/:ratio/read/:reads", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	ap.Router().Handle("/benchmark/sqlite/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	//ap.Router().Handle("/benchmark/sqlite/pool/ratio/:ratio/read/:reads", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	ap.Router().Handle("GET /benchmark/sqlite/pool/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	// This is an example of init function
	ap.Router().Handle("/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	ap.Router().Handle("/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
	ap.Router().Handle("GET /oauth2-providers", commonMiddleware.ThenFunc(ap.OAuth2ProvidersHandler))
}
