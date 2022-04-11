package main

import (
    "encoding/json"
    "net/http"
    "math/rand"
    "strconv"
)

func (c *App) adminHandler(w http.ResponseWriter, r *http.Request) {

    user := "testuser"
    //user := context.Get(r, "user")
    // Maybe other operations on the database
    json.NewEncoder(w).Encode(user)
}


func (c *App) teaHandler(w http.ResponseWriter, r *http.Request) {
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
