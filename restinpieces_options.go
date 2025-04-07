package restinpieces

// this shoudl be under custom???
// just initilizes the custom packges that implments the app
import (
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/db/crawshaw"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/router/httprouter"
	"github.com/caasmo/restinpieces/router/servemux"
	crawshawPool "crawshaw.io/sqlite/sqlitex" // Alias for crawshaw pool type
	phuslog "github.com/phuslu/log"
	zombiezenPool "zombiezen.com/go/sqlite/sqlitex" // Alias for zombiezen pool type
)


// WithExistingCrawshawPool configures the App to use an existing Crawshaw SQLite pool.
func WithExistingCrawshawPool(pool *crawshawPool.Pool) core.Option {
	dbInstance, err := crawshaw.NewWithPool(pool)
	if err != nil {
		// Handle error appropriately - perhaps panic or log fatal
		// For now, let's panic as this indicates a setup issue.
		panic(fmt.Sprintf("failed to initialize crawshaw DB with existing pool: %v", err))
	}
	// Assuming WithDB is replaced by WithDbProvider later
	// return core.WithDB(dbInstance)
	// Placeholder until WithDbProvider is introduced:
	return func(a *core.App) {
		// This assignment will need to change when App uses DbProvider
		a.SetDb(dbInstance) // Assuming a temporary SetDb method exists or is added
	}
}

// WithExistingZombiezenPool configures the App to use an existing Zombiezen SQLite pool.
func WithExistingZombiezenPool(pool *zombiezenPool.Pool) core.Option {
	dbInstance, err := zombiezen.NewWithPool(pool)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize zombiezen DB with existing pool: %v", err))
	}
	// Assuming WithDB is replaced by WithDbProvider later
	// return core.WithDB(dbInstance)
	// Placeholder until WithDbProvider is introduced:
	return func(a *core.App) {
		// This assignment will need to change when App uses DbProvider
		a.SetDb(dbInstance) // Assuming a temporary SetDb method exists or is added
	}
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
	cacheInstance, err := ristretto.New[any]() // Explicit string keys and interface{} values
	if err != nil {
		panic(fmt.Sprintf("failed to initialize ristretto cache: %v", err))
	}
	return core.WithCache(cacheInstance)
}

// DefaultLoggerOptions provides default settings for slog handlers.
// Level: Debug, Removes time and level attributes from output.
var DefaultLoggerOptions = &slog.HandlerOptions{
	Level: slog.LevelDebug,
	ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		//if a.Key == slog.TimeKey || a.Key == slog.LevelKey {
		if a.Key == slog.TimeKey {
			return slog.Attr{} // Return empty Attr to remove
		}
		return a
	},
}

// WithPhusLog configures slog with phuslu/log's JSON handler.
// Uses DefaultLoggerOptions if opts is nil.
func WithPhusLogger(opts *slog.HandlerOptions) core.Option {
	if opts == nil {
		opts = DefaultLoggerOptions // Use package-level defaults
	}
	logger := slog.New(phuslog.SlogNewJSONHandler(os.Stderr, opts))

	// TODO remove slog.SetDefault call? It affects global state.
	slog.SetDefault(logger)
	return core.WithLogger(logger)
}

// WithTextHandler configures slog with the standard library's text handler.
func WithTextLogger(opts *slog.HandlerOptions) core.Option {
	// Ensure opts is not nil to avoid panic
	if opts == nil {
		opts = DefaultLoggerOptions // Use package-level defaults
	}
	// Use os.Stdout for text logs, os.Stderr for JSON logs is common practice
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	return core.WithLogger(logger)
}
