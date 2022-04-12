package main

import (
    "log"
    "net/http"

    "github.com/justinas/alice"
    "github.com/caasmo/restinpieces/router"
    "github.com/caasmo/restinpieces/db"
)

func main() {

	db, err := db.New("bench.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

    app := NewApp(db)

    commonHandlers := alice.New(app.loggingMw)
    router := router.New()
    router.Get("/admin", commonHandlers.Append(app.authMw).ThenFunc(app.adminHdl))
    router.Get("/about", commonHandlers.ThenFunc(app.aboutHdl))
    router.Get("/", commonHandlers.ThenFunc(app.indexHdl))
    router.Get("/db", commonHandlers.ThenFunc(app.testDbHdl))
    router.Get("/teas/:id", commonHandlers.ThenFunc(app.teaHdl))
    log.Fatal(http.ListenAndServe(":8080", router))
}
