package main

import (
	"github.com/caasmo/restinpieces/db"
)

// App is the application wide context.
// db connections and permanent structs should go here.
//
// For simplicity, all handlers and middleware should have App as receiver.
type App struct {
	dbase *db.Db
}

func NewApp(d *db.Db) *App {
	return &App{dbase: d}
}
