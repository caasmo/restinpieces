package app

import (
	"fmt"
	"time"
	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router"
)

// App is the application wide context.
// db connections and permanent structs should go here.
//
// For simplicity, all handlers and middleware should have App as receiver.
// That why App needs to be in the same package "main" as the handlers.

type App struct {
	db          db.Db
	router      router.Router
	cache       cache.Cache
	config      *Config
}

// TODO move
type Config struct {
	JwtSecret     []byte
	TokenDuration time.Duration
}

type Option func(*App)

// WithCache sets the cache implementation
func WithCache(c cache.Cache) Option {
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

// WithConfig sets the application configuration
func WithConfig(cfg *Config) Option {
	return func(a *App) {
		a.config = cfg
	}
}

func New(opts ...Option) (*App, error) {
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
	if a.config == nil {
		return nil, fmt.Errorf("config is required but was not provided")
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
