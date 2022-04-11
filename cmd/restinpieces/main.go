package main

import (
    "log"
    "net/http"

    "github.com/justinas/alice"
    "github.com/caasmo/restinpieces/router"
    "github.com/caasmo/restinpieces/db"
)

func main() {

	//poolSize runtime.NumCPU()/2
	db, err := db.New("bench.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

    app := App{db}

    commonHandlers := alice.New(loggingHandler)
    router := router.New()
    router.Get("/admin", commonHandlers.Append(app.authHandler).ThenFunc(app.adminHandler))
    router.Get("/about", commonHandlers.ThenFunc(aboutHandler))
    router.Get("/", commonHandlers.ThenFunc(indexHandler))
    router.Get("/db", commonHandlers.ThenFunc(app.testDb))
    router.Get("/teas/:id", commonHandlers.ThenFunc(app.teaHandler))
    log.Fatal(http.ListenAndServe(":8080", router))
}
