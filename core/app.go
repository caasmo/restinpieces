package core

import (
	"fmt"
	"log/slog"
	//"sync/atomic" // No longer needed here, moved to config.Provider

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
	db             db.Db
	router         router.Router
	cache          cache.Cache[string, interface{}] // Using string keys and interface{} values
	configProvider *config.Provider                 // Holds the config provider
	logger         *slog.Logger
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
	// Check if config provider was set via options
	if a.configProvider == nil {
		return nil, fmt.Errorf("config provider is required but was not provided via WithConfigProvider option")
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

// Config returns the currently active application config instance
// by retrieving it from the config provider.
func (a *App) Config() *config.Config {
	// Delegate fetching the config to the provider
	return a.configProvider.Get()
}
