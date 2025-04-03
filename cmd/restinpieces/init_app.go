package main

import (
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces/core"
	// TOD0 problem cgo compile check?
	"github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db/crawshaw"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/router/httprouter"
	"github.com/caasmo/restinpieces/router/servemux"
)

func WithDBCrawshaw(dbPath string) core.Option {
	db, _ := crawshaw.New(dbPath)
	// TODO erro log fatal

	return core.WithDB(db)
}

func WithDBZombiezen() core.Option {

	db, _ := zombiezen.New("bench.db")
	// TODO erro log fatal

	return core.WithDB(db)
}

func WithRouterServeMux() core.Option {
	r := servemux.New()
	return core.WithRouter(r)
}

func WithRouterHttprouter() core.Option {
	r := httprouter.New()
	return core.WithRouter(r)
}

func WithCacheRistretto() core.Option {
	cache, _ := ristretto.New[string, interface{}]() // Explicit string keys and interface{} values
	// TODO fatal
	return core.WithCache(cache)
}

func initApp(cfg *config.Config) (*core.App, error) {
	// Initialize a default logger
	// TODO: Make logger configuration more flexible (e.g., JSON handler, level)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	return core.NewApp(
		WithDBCrawshaw(cfg.DBFile),
		WithRouterServeMux(),
		WithCacheRistretto(),
		core.WithConfig(cfg),
		core.WithLogger(logger), // Provide the logger
	)
}
