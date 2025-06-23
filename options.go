package restinpieces

import (
	"log/slog"

	"filippo.io/age"
	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/notify"
	"github.com/caasmo/restinpieces/router"
)

type Option func(*initializer)

// WithCache sets the cache implementation
func WithCache(c cache.Cache[string, interface{}]) Option {
	return func(i *initializer) {
		i.app.cache = c
	}
}

// WithDbApp sets the application's database implementation.
// It expects a single concrete type (like *crawshaw.Db) that implements db.DbApp.
func WithDbApp(dbApp db.DbApp) Option {
	return func(i *initializer) {
		if dbApp == nil {
			panic("DbApp cannot be nil")
		}
		i.app.dbAuth = dbApp
		i.app.dbQueue = dbApp
		i.app.dbConfig = dbApp
	}
}

// WithRouter sets the router implementation
func WithRouter(r router.Router) Option {
	return func(i *initializer) {
		i.app.router = r
	}
}

// WithLogger sets the logger implementation
func WithLogger(l *slog.Logger) Option {
	return func(i *initializer) {
		i.app.logger = l
	}
}

// WithAgeKeyPath sets the path to the age identity file
func WithAgeKeyPath(path string) Option {
	return func(i *initializer) {
		i.app.ageKeyPath = path
	}
}

// WithNotifier sets the notifier implementation
func WithNotifier(n notify.Notifier) Option {
	return func(i *initializer) {
		i.app.notifier = n
	}
}
