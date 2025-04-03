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
func WithPhusLogger(opts *slog.HandlerOptions) core.Option {
	// Ensure opts is not nil to avoid panic
	if opts == nil {
		opts = &slog.HandlerOptions{} // Use default options if nil
	}

	// If ReplaceAttr is already set, chain the removal logic.
	// Otherwise, just set the removal logic.
	originalReplaceAttr := opts.ReplaceAttr
	opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
		// Apply original ReplaceAttr first if it exists
		if originalReplaceAttr != nil {
			a = originalReplaceAttr(groups, a)
			// If the original function removed the attribute, respect that
			if a.Key == "" {
				return a
			}
		}
		// Remove time and level attributes from the output
		if a.Key == slog.TimeKey || a.Key == slog.LevelKey {
				return slog.Attr{} // Return an empty Attr to remove it
			}
			return a
		},
		// AddSource: true, // Uncomment if you want source file/line info
	}
	logger := slog.New(phuslog.SlogNewJSONHandler(os.Stderr, opts))

	// TODO remove
	slog.SetDefault(logger)
	return core.WithLogger(logger)
}

// WithTextHandler configures slog with the standard library's text handler.
func WithTextHandler(opts *slog.HandlerOptions) core.Option {
	// Ensure opts is not nil to avoid panic
	if opts == nil {
		opts = &slog.HandlerOptions{} // Use default options if nil
	}
	// Use os.Stdout for text logs, os.Stderr for JSON logs is common practice
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	return core.WithLogger(logger)
}

func initApp(cfg *config.Config) (*core.App, error) {

	return core.NewApp(
		WithDBCrawshaw(cfg.DBFile),
		WithRouterServeMux(),
		WithCacheRistretto(),
		core.WithConfig(cfg),
		// Create HandlerOptions here
		WithPhusLogger(&slog.HandlerOptions{
			Level: slog.LevelDebug,
			// AddSource: true, // Example: enable source info
		}), // Provide the logger
	)
}
