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

// WithDB sets the database implementation (DEPRECATED - Use WithDbProvider or specific WithExisting...Pool options)
// Keeping this temporarily for compatibility during refactor.
func WithDB(d db.Db) Option {
	return func(a *App) {
		a.db = d
	}
}

// SetDb is a temporary method for the placeholder in restinpieces_options.go
// Remove this once WithDbProvider is fully implemented and used.
func (a *App) SetDb(d db.Db) {
	a.db = d
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
