package app

import (
	"github.com/caasmo/restinpieces/cache"
	dbIface "github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
)

// App is the application wide context.
// db connections and permanent structs should go here.
//
// For simplicity, all handlers and middleware should have App as receiver.
// That why App needs to be in the same package "main" as the handlers.
type App struct {
	db          *dbIface.Db
	router      router.Router
	routerParam router.ParamGeter
	cache       cache.Cache
}

// just 1 method
// params =+ app.NamedParams.Get(ctx Context)
// param.ByName(ctx Context, name)

func New(d *dbIface.Db, r router.Router, p router.ParamGeter, c cache.Cache) *App {
	return &App{db: d, router: r, routerParam: p, cache: c}
}

// Router returns the application's router instance
func (a *App) Router() router.Router {
	return a.router
}

// Close all
func (a *App) Close() {
	a.db.Close()
}
