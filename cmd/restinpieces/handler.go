package main

import (
    "log"
    "fmt"
    "net/http"
    "time"
)

func loggingHandler(next http.Handler) http.Handler {
    fn := func(w http.ResponseWriter, r *http.Request) {
        t1 := time.Now()
        next.ServeHTTP(w, r)
        t2 := time.Now()
        log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), t2.Sub(t1))
    }

    return http.HandlerFunc(fn)
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "You are on the about page.")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Welcome!")
}

