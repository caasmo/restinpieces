package main

import (
    "encoding/json"
    "net/http"
    "log"
)

func (c *App) adminHandler(w http.ResponseWriter, r *http.Request) {

    user := "testuser"
    //user := context.Get(r, "user")
    // Maybe other operations on the database
    json.NewEncoder(w).Encode(user)
}

func (c *App) authHandler(next http.Handler) http.Handler {
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

func (c *App) teaHandler(w http.ResponseWriter, r *http.Request) {
    //params := context.Get(r, "params").(httprouter.Params)
    //log.Println(params.ByName("id"))
    // tea := getTea(c.db, params.ByName("id"))
    json.NewEncoder(w).Encode(nil)
}
