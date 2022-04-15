package main

import (
	"log"
	"net/http"

	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
	"github.com/justinas/alice"
)

func main() {

	db, err := db.New("bench.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	app := NewApp(db)

	commonHandlers := alice.New(app.logging)
	noMiddleware := alice.New()
	router := router.New()
	router.Get("/admin", commonHandlers.Append(app.auth).ThenFunc(app.admin))
	router.Get("/about", commonHandlers.ThenFunc(app.about))
	router.Get("/", commonHandlers.ThenFunc(app.index))
	router.Get("/example/sqlite/read/randompk", noMiddleware.ThenFunc(app.exampleSqliteReadRandom))
	router.Get("/example/sqlite/writeone/:value", noMiddleware.ThenFunc(app.exampleWriteOne))
	router.Get("/benchmark/sqlite/ratio/:ratio/read/:reads", noMiddleware.ThenFunc(app.benchmarkSqliteRWRatio))
	router.Get("/benchmark/sqlite/pool/ratio/:ratio/read/:reads", noMiddleware.ThenFunc(app.benchmarkSqliteRWRatioPool))
	router.Get("/teas/:id", commonHandlers.ThenFunc(app.tea))
	log.Fatal(http.ListenAndServe(":8080", router))
}
