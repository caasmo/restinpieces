package core

import (
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
)

// App is the application wide context.
// db connections and permanent structs should go here.
//
// For simplicity, all handlers and middleware should have App as receiver.
// That why App needs to be in the same package "main" as the handlers.

// app is a service with heavy objects for the handlers.
// and also a out the box coded endpoints handlers. (methods)
type App struct {
	db     db.Db
	router router.Router
	cache  cache.Cache[string, interface{}] // Using string keys and interface{} values
	config atomic.Value                     // Holds *config.Config, allows atomic swaps
	logger *slog.Logger
	//proxy *proxy.Proxy
}


func NewApp(opts ...Option) (*App, error) {
	a := &App{}
	for _, opt := range opts {
		opt(a)
	}

	if a.db == nil {
		return nil, fmt.Errorf("db is required but was not provided")
	}
	if a.router == nil {
		return nil, fmt.Errorf("router is required but was not provided")
	}
	// Check if config was initialized via options by loading from atomic.Value
	if a.config.Load() == nil {
		// WithConfig option should have stored the initial config.
		// If it's still nil here, it means WithConfig wasn't used or passed a nil config.
		return nil, fmt.Errorf("config is required but was not provided via WithConfig option")
	}
	if a.logger == nil {
		// Default to slog.Default() if no logger is provided? Or require it?
		// Let's require it for now for explicitness.
		return nil, fmt.Errorf("logger is required but was not provided")
	}

	return a, nil
}

// Router returns the application's router instance
func (a *App) Router() router.Router {
	return a.router
}

// Close all
func (a *App) Close() {
	a.db.Close()
}

// Db returns the database instance
func (a *App) Db() db.Db {
	return a.db
}

// Logger returns the application's logger instance
func (a *App) Logger() *slog.Logger {
	return a.logger
}

// Cache returns the application's cache instance
func (a *App) Cache() cache.Cache[string, interface{}] {
	return a.cache
}

// Config returns the currently active application config instance.
// It safely loads the config from the atomic value.
func (a *App) Config() *config.Config {
	// Load returns an interface{}, so we need to assert the type.
	// This is safe because we ensure only *config.Config is stored via SetConfig and WithConfig.
	cfg := a.config.Load().(*config.Config)
	return cfg
}

// SetConfig atomically updates the application's configuration.
// This is intended to be used for hot reloading (e.g., on SIGHUP).
func (a *App) SetConfig(newCfg *config.Config) {
	if newCfg == nil {
		a.logger.Error("attempted to set nil configuration")
		return // Or handle as appropriate, maybe panic?
	}
	a.config.Store(newCfg)
	a.logger.Info("configuration reloaded successfully")
}

// SetProxy sets the proxy instance on the App.
// This is typically called after App initialization to resolve circular dependencies.
//func (a *App) SetProxy(p *proxy.Proxy) {
//	a.proxy = p
//}

// Proxy returns the application's proxy instance
// It might panic if SetProxy was not called after NewApp.
//func (a *App) Proxy() *proxy.Proxy {
//	return a.proxy
//}
