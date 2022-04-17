package main

import (
	"log"
	"net/http"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/app"
	router "github.com/caasmo/restinpieces/router/httprouter"
	"github.com/justinas/alice"
)

func main() {

	db, err := db.New("bench.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rp := router.NewParamGeter()
	ap := app.New(db, rp)

	commonMiddleware := alice.New(ap.Logger)


	router := router.New()
	router.Get("/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	router.Get("/", commonMiddleware.ThenFunc(ap.Index))
	router.Get("/example/sqlite/read/randompk",http.HandlerFunc(ap.ExampleSqliteReadRandom))
	router.Get("/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	router.Get("/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	router.Get("/benchmark/sqlite/ratio/:ratio/read/:reads",http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	router.Get("/benchmark/sqlite/pool/ratio/:ratio/read/:reads", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	router.Get("/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
	log.Fatal(http.ListenAndServe(":8080", router))
}
