package main

import (
	"github.com/caasmo/restinpieces/app"
	"github.com/caasmo/restinpieces/router"
	"github.com/justinas/alice"
	"net/http"
)

func route(r router.Router, ap *app.App) {
	commonMiddleware := alice.New(ap.Logger)
	r.Handle("/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	r.Handle("/", commonMiddleware.ThenFunc(ap.Index))
	r.Handle("/example/sqlite/read/randompk", http.HandlerFunc(ap.ExampleSqliteReadRandom))
	r.Handle("/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	//router.Handle("/example/ristretto/writeread/:value", http.HandlerFunc(ap.ExampleRistrettoWriteRead))
	r.Handle("/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	r.Handle("/benchmark/sqlite/ratio/:ratio/read/:reads", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	r.Handle("/benchmark/sqlite/pool/ratio/:ratio/read/:reads", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	// This is an example of init function
	r.Handle("/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	r.Handle("/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
}
