package main

import (
    "fmt"
    "net/http"
)


func aboutHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "You are on the about page.")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Welcome!")
}

