package restinpieces

import (
	"log/slog"

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
		i.app.SetCache(c)
	}
}

// WithDbApp sets the application's database implementation.
// It expects a single concrete type (like *crawshaw.Db) that implements db.DbApp.
func WithDbApp(dbApp db.DbApp) Option {
	return func(i *initializer) {
		i.app.SetDb(dbApp)
	}
}

// WithRouter sets the router implementation
func WithRouter(r router.Router) Option {
	return func(i *initializer) {
		i.app.SetRouter(r)
	}
}

// WithLogger sets the logger implementation
func WithLogger(l *slog.Logger) Option {
	return func(i *initializer) {
		i.app.SetLogger(l)
	}
}

// WithAgeKeyPath sets the path to the age identity file
func WithAgeKeyPath(path string) Option {
	return func(i *initializer) {
		i.app.SetAgeKeyPath(path)
	}
}

// WithNotifier sets the notifier implementation
func WithNotifier(n notify.Notifier) Option {
	return func(i *initializer) {
		i.app.SetNotifier(n)
	}
}
