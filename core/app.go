package core

import (
	"fmt"
	"log/slog"

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
	dbAuth   db.DbAuth
	dbQueue  db.DbQueue
	dbConfig db.DbConfig
	router router.Router
	cache          cache.Cache[string, interface{}] // Using string keys and interface{} values
	configProvider *config.Provider                 // Holds the config provider
	logger         *slog.Logger
	ageKeyPath     string                           // Path to age identity file
	secureConfig   config.SecureConfig              // Secure configuration handler
}

// ServeHTTP method removed as App no longer acts as the primary handler

func NewApp(opts ...Option) (*App, error) {
	a := &App{}
	for _, opt := range opts {
		opt(a)
	}

	// Check for required interfaces
	if a.dbAuth == nil {
		return nil, fmt.Errorf("dbAuth is required but was not provided (use WithDbApp)")
	}
	if a.dbQueue == nil {
		return nil, fmt.Errorf("dbQueue is required but was not provided (use WithDbApp)")
	}

	if a.router == nil {
		return nil, fmt.Errorf("router is required but was not provided")
	}

	if a.logger == nil {
		return nil, fmt.Errorf("logger is required but was not provided")
	}

    sc, err := config.NewSecureConfigAge(a.dbConfig, a.ageKeyPath, a.logger)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize secure config: %w", err)
    }

    a.secureConfig = sc

	return a, nil
}

// Router returns the application's router instance
func (a *App) Router() router.Router {
	return a.router
}

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
// TODO remove? 
func (a *App) Config() *config.Config {
	// Delegate fetching the config to the provider
	return a.configProvider.Get()
}

// SecureConfig returns the application's secure configuration handler
func (a *App) SecureConfigStore() config.SecureConfig {
	return a.secureConfig
}

// SetConfigProvider allows setting the config provider after App initialization.
func (a *App) SetConfigProvider(provider *config.Provider) {
	a.configProvider = provider
}
