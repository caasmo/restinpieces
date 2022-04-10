package main

import (
    "log"
    "net/http"

    "github.com/justinas/alice"
    "github.com/caasmo/restinpieces/router"
)

func main() {
    app := App{nil}
    commonHandlers := alice.New(loggingHandler)
    router := router.New()
    router.Get("/admin", commonHandlers.Append(app.authHandler).ThenFunc(app.adminHandler))
    router.Get("/about", commonHandlers.ThenFunc(aboutHandler))
    router.Get("/", commonHandlers.ThenFunc(indexHandler))
    router.Get("/teas/:id", commonHandlers.ThenFunc(app.teaHandler))
    log.Fatal(http.ListenAndServe(":8080", router))
}
