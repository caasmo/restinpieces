package main

import (
	"github.com/caasmo/restinpieces/app"
	"github.com/caasmo/restinpieces/router"
	"github.com/justinas/alice"
	"net/http"
)

func route(r router.Router, ap *app.App) {
	commonMiddleware := alice.New(ap.Logger)
	r.Get("/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	r.Get("/", commonMiddleware.ThenFunc(ap.Index))
	r.Get("/example/sqlite/read/randompk", http.HandlerFunc(ap.ExampleSqliteReadRandom))
	r.Get("/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	//router.Get("/example/ristretto/writeread/:value", http.HandlerFunc(ap.ExampleRistrettoWriteRead))
	r.Get("/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	r.Get("/benchmark/sqlite/ratio/:ratio/read/:reads", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	r.Get("/benchmark/sqlite/pool/ratio/:ratio/read/:reads", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	// This is an example of init function
	r.Get("/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	r.Get("/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
}
