package core

import (
	"log/slog"

	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/notify"
	"github.com/caasmo/restinpieces/router"
)

type Option func(*App)

// WithCache sets the cache implementation
func WithCache(c cache.Cache[string, interface{}]) Option {
	return func(a *App) {
		a.cache = c
	}
}

// WithDbApp sets the application's database implementation.
// It expects a single concrete type (like *crawshaw.Db) that implements db.DbApp.
func WithDbApp(dbApp db.DbApp) Option {
	return func(a *App) {
		if dbApp == nil {
			panic("DbApp cannot be nil")
		}
		a.dbAuth = dbApp
		a.dbQueue = dbApp
		a.dbConfig = dbApp
	}
}

// WithRouter sets the router implementation
func WithRouter(r router.Router) Option {
	return func(a *App) {
		a.router = r
	}
}

// WithLogger sets the logger implementation
func WithLogger(l *slog.Logger) Option {
	return func(a *App) {
		a.logger = l
	}
}

// WithAgeKeyPath sets the path to the age identity file
func WithAgeKeyPath(path string) Option {
	return func(a *App) {
		a.ageKeyPath = path
	}
}

// WithNotifier sets the notifier implementation
func WithNotifier(n notify.Notifier) Option {
	return func(a *App) {
		a.notifier = n
	}
}
