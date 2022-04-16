package main

import (
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
)

// App is the application wide context.
// db connections and permanent structs should go here.
//
// For simplicity, all handlers and middleware should have App as receiver.
// That why App needs to be in the same package "main" as the handlers.
type App struct {
	dbase   *db.Db
	nParams router.NamedParams
}

// just 1 method
// params =+ app.NamedParams.Get(ctx Context)
// param.ByName(ctx Context, name)

func NewApp(d *db.Db, p router.NamedParams) *App {
	return &App{dbase: d, nParams: p}
}
