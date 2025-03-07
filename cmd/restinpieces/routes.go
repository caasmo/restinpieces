package main

import (
	"github.com/caasmo/restinpieces/app"
	"github.com/justinas/alice"
	"net/http"
)

func route(ap *app.App) {
	commonMiddleware := alice.New(ap.SecurityHeadersMiddleware, ap.Logger)
	authMiddleware := alice.New(ap.JwtValidate)
	ap.Router().Handle("POST /auth-refresh", authMiddleware.ThenFunc(ap.RefreshAuthHandler))
	ap.Router().Handle("POST /auth-with-password", commonMiddleware.ThenFunc(ap.AuthWithPasswordHandler))
	ap.Router().Handle("/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	ap.Router().Handle("/", commonMiddleware.ThenFunc(ap.Index))
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
}
