package restinpieces

// this shoudl be under custom???
// just initilizes the custom packges that implments the app
import (
	"fmt"
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/core"
	phuslog "github.com/phuslu/log"
)


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



func WithTextLogger(opts *slog.HandlerOptions) core.Option {
	// Ensure opts is not nil to avoid panic
	if opts == nil {
		opts = DefaultLoggerOptions // Use package-level defaults
	}
	// Use os.Stdout for text logs, os.Stderr for JSON logs is common practice
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	return core.WithLogger(logger)
}
