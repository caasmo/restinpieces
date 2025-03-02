package app

import (
	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
)

// App is the application wide context.
// db connections and permanent structs should go here.
//
// For simplicity, all handlers and middleware should have App as receiver.
// That why App needs to be in the same package "main" as the handlers.
type App struct {
	db          *db.Db
	router      router.Router
	cache       cache.Cache
}

type AppOption func(*App)

// WithCache sets the cache implementation
func WithCache(c cache.Cache) AppOption {
	return func(a *App) {
		a.cache = c
	}
}

// just 1 method
// params =+ app.NamedParams.Get(ctx Context)
// param.ByName(ctx Context, name)

func New(d *db.Db, r router.Router, opts ...AppOption) *App {
	a := &App{db: d, router: r}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Router returns the application's router instance
func (a *App) Router() router.Router {
	return a.router
}

// Close all
func (a *App) Close() {
	a.db.Close()
}
