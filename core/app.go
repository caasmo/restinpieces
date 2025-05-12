package core

import (
	"fmt"
	"log/slog"

	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/notify"
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
	dbAuth         db.DbAuth
	dbQueue        db.DbQueue
	dbConfig       db.DbConfig
	router         router.Router
	cache          cache.Cache[string, interface{}] // Using string keys and interface{} values
	configProvider *config.Provider                 // Holds the config provider
	logger         *slog.Logger
	ageKeyPath     string // Path to age identity file
	// SecureStore provides encrypted configuration storage/retrieval capabilities.
	// It acts as a secrets manager for securely storing sensitive configuration in the database.
	// Primarily used during application initialization to load encrypted configs on startup.
	// The implementation uses age encryption with keys from ageKeyPath.
	secureStore config.SecureStore
	notifier    notify.Notifier
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
	if a.dbConfig == nil {
		return nil, fmt.Errorf("dbConfig is required but was not provided (use WithDbApp)")
	}

	if a.ageKeyPath == "" {
		return nil, fmt.Errorf("ageKeyPath is required but was not provided (use WithAgeKeyPath)")
	}

	ss, err := config.NewSecureStoreAge(a.dbConfig, a.ageKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secure store: %w", err)
	}

	a.secureStore = ss

	return a, nil
}

// Router returns the application's router instance
func (a *App) Router() router.Router {
	return a.router
}

func (a *App) SetRouter(r router.Router) {
	a.router = r
}

func (a *App) DbAuth() db.DbAuth {
	return a.dbAuth
}

func (a *App) DbQueue() db.DbQueue {
	return a.dbQueue
}

func (a *App) Logger() *slog.Logger {
	return a.logger
}

func (a *App) SetLogger(l *slog.Logger) {
	a.logger = l
}

func (a *App) SetCache(c cache.Cache[string, interface{}]) {
	a.cache = c
}

func (a *App) Cache() cache.Cache[string, interface{}] {
	return a.cache
}

func (a *App) Config() *config.Config {
	return a.configProvider.Get()
}

func (a *App) SecureStore() config.SecureStore {
	return a.secureStore
}

func (a *App) Notifier() notify.Notifier {
	return a.notifier
}

func (a *App) SetNotifier(n notify.Notifier) {
	a.notifier = n
}

func (a *App) SetConfigProvider(provider *config.Provider) {
	a.configProvider = provider
}
