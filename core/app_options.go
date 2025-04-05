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

// WithDB sets the database implementation
func WithDB(d db.Db) Option {
	return func(a *App) {
		a.db = d
	}
}

// WithRouter sets the router implementation
func WithRouter(r router.Router) Option {
	return func(a *App) {
		a.router = r
	}
}

// WithConfig sets the initial application configuration.
// It stores the provided config into the atomic.Value.
func WithConfig(cfg *config.Config) Option {
	return func(a *App) {
		if cfg == nil {
			// Handle nil config case if necessary, maybe panic or log
			// For now, let's assume a valid config is always provided initially.
			// If not, NewApp will return an error later.
			return // Or panic("initial config cannot be nil")
		}
		a.config.Store(cfg)
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

