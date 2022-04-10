package main

import (
    "database/sql"
    "encoding/json"
    //"errors"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/justinas/alice"

    "github.com/caasmo/restinpieces/router"
)

type appContext struct {
    db *sql.DB
}

func loggingHandler(next http.Handler) http.Handler {
    fn := func(w http.ResponseWriter, r *http.Request) {
        t1 := time.Now()
        next.ServeHTTP(w, r)
        t2 := time.Now()
        log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), t2.Sub(t1))
    }

    return http.HandlerFunc(fn)
}

func (c *appContext) adminHandler(w http.ResponseWriter, r *http.Request) {

    user := "testuser"
    //user := context.Get(r, "user")
    // Maybe other operations on the database
    json.NewEncoder(w).Encode(user)
}

func (c *appContext) authHandler(next http.Handler) http.Handler {
    fn := func(w http.ResponseWriter, r *http.Request) {
        authToken := r.Header.Get("Authorization")
        //user, err := map[string]interface{}{}, errors.New("test")
        //user := authToken
        // user, err := getUser(c.db, authToken)
        log.Println(authToken)

        // if return in middleware, no next, chain stopped
        //if err != nil {
        //    http.Error(w, http.StatusText(401), 401)
        //    return
        //}

        // TODO communication betwwen handlers
        //context.Set(r, "user", user)
        next.ServeHTTP(w, r)
    }

    return http.HandlerFunc(fn)
}

func (c *appContext) teaHandler(w http.ResponseWriter, r *http.Request) {
    //params := context.Get(r, "params").(httprouter.Params)
    //log.Println(params.ByName("id"))
    // tea := getTea(c.db, params.ByName("id"))
    json.NewEncoder(w).Encode(nil)
}


func aboutHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "You are on the about page.")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Welcome!")
}

func main() {
    appC := appContext{nil}
    commonHandlers := alice.New(loggingHandler)
    router := router.New()
    router.Get("/admin", commonHandlers.Append(appC.authHandler).ThenFunc(appC.adminHandler))
    router.Get("/about", commonHandlers.ThenFunc(aboutHandler))
    router.Get("/", commonHandlers.ThenFunc(indexHandler))
    router.Get("/teas/:id", commonHandlers.ThenFunc(appC.teaHandler))
    log.Fatal(http.ListenAndServe(":8080", router))
}
