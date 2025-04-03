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
	phuslog "github.com/phuslu/log"
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

// WithPhusLog configures slog with phuslu/log's JSON handler.
func WithPhusLog(level slog.Level) core.Option {
	logger := slog.New(phuslog.SlogNewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
		// AddSource: true, // Uncomment if you want source file/line info
	}))
	return core.WithLogger(logger)
}

func initApp(cfg *config.Config) (*core.App, error) {

	return core.NewApp(
		WithDBCrawshaw(cfg.DBFile),
		WithRouterServeMux(),
		WithCacheRistretto(),
		core.WithConfig(cfg),
        WithPhusLog(slog.LevelInfo), // Provide the logger
	)
}
