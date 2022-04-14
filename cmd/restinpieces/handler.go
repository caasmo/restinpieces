package main

import (
    "encoding/json"
    "net/http"
    "fmt"
    "math/rand"
    "strconv"
)

// all handlers should conform to fn(w http.ResponseWriter, r *http.Request) 
//
// Differentiate from the Handler by using suffix 

func (c *App) admin(w http.ResponseWriter, r *http.Request) {

    user := "testuser"
    //user := context.Get(r, "user")
    // Maybe other operations on the database
    json.NewEncoder(w).Encode(user)
}


func (c *App) tea(w http.ResponseWriter, r *http.Request) {
    //params := context.Get(r, "params").(httprouter.Params)
    //log.Println(params.ByName("id"))
    // tea := getTea(c.db, params.ByName("id"))
    json.NewEncoder(w).Encode(nil)
}

func (c *App) testDb(w http.ResponseWriter, r *http.Request) {

    id := rand.Intn(100000)+1
    value := c.dbase.GetById(id)
	w.Write([]byte(`{"id":` + strconv.Itoa(id) + `,"value":` + strconv.Itoa(value) + `}`))
}


func (c *App) about(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "You are on the about page.")
}

func (c *App) index(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Welcome!")
}

