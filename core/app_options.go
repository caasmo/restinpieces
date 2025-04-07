package core

import (
	"log/slog"

	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
)

type Option func(*App)

// WithCache sets the cache implementation
func WithCache(c cache.Cache[string, interface{}]) Option {
	return func(a *App) {
		a.cache = c
	}
}

// DbProvider is an interface combining the required DB roles.
// The concrete DB implementation (e.g., *crawshaw.Db) must satisfy this interface.
type DbProvider interface {
	db.DbAuth
	db.DbQueue
	db.DbLifecycle
}

// WithDbProvider sets the database providers (Auth, Queue, Lifecycle) in the App.
// It expects a single concrete type (like *crawshaw.Db) that implements DbProvider.
func WithDbProvider(provider DbProvider) Option {
	return func(a *App) {
		if provider == nil {
			// Or panic, depending on desired behavior for nil provider
			// This helps catch errors early during setup.
			panic("DbProvider cannot be nil")
		}
		a.dbAuth = provider
		a.dbQueue = provider
		a.dbLifecycle = provider
	}
}

// WithRouter sets the router implementation
func WithRouter(r router.Router) Option {
	return func(a *App) {
		a.router = r
	}
}

// WithConfigProvider sets the application's configuration provider.
func WithConfigProvider(p *config.Provider) Option {
	return func(a *App) {
		a.configProvider = p
	}
}

// WithLogger sets the logger implementation
func WithLogger(l *slog.Logger) Option {
	return func(a *App) {
		a.logger = l
	}
}

// TODO
// WithProxy sets the proxy implementation
//func WithProxy(p *proxy.Proxy) Option {
//	return func(a *App) {
//		a.proxy = p
//	}
//}
