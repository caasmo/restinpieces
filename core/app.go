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
	dbAuth  db.DbAuth
	dbQueue db.DbQueue
	// dbLifecycle    db.DbLifecycle // Removed: Lifecycle managed externally
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

	// Check for the new required interfaces
	if a.dbAuth == nil {
		return nil, fmt.Errorf("dbAuth is required but was not provided (use WithDbProvider)")
	}
	if a.dbQueue == nil {
		return nil, fmt.Errorf("dbQueue is required but was not provided (use WithDbProvider)")
	}
	// dbLifecycle check removed
	// Check other required dependencies
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

// Close method removed as DB lifecycle is managed externally.

// AuthDb returns the DbAuth interface implementation for authentication operations.
func (a *App) DbAuth() db.DbAuth {
	return a.dbAuth
}

// QueueDb returns the DbQueue interface implementation for job queue operations.
func (a *App) DbQueue() db.DbQueue {
	return a.dbQueue
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

// SetConfigProvider allows setting the config provider after App initialization.
func (a *App) SetConfigProvider(provider *config.Provider) {
	a.configProvider = provider
}
