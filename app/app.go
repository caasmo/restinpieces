package app

import (
	dbIface "github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
	"github.com/caasmo/restinpieces/cache"
)

// App is the application wide context.
// db connections and permanent structs should go here.
//
// For simplicity, all handlers and middleware should have App as receiver.
// That why App needs to be in the same package "main" as the handlers.
type App struct {
	db          *dbIface.Db
	routerParam router.ParamGeter
	cache          cache.Cache

}

// just 1 method
// params =+ app.NamedParams.Get(ctx Context)
// param.ByName(ctx Context, name)

func New(d *dbIface.Db, p router.ParamGeter, c cache.Cache) *App {
    return &App{db: d, routerParam: p, cache: c}
}
