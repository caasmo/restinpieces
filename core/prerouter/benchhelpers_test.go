package prerouter

import (
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"

	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

// noOpHandler is a simple http.Handler that does nothing, used as the final handler in chains.
var noOpHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

// newBenchmarkApp creates a mock core.App with a configurable setup for benchmarking.
// It uses a discard logger and a fresh cache for each benchmark run to ensure isolation.
func newBenchmarkApp(b *testing.B, cfgModifiers ...func(*config.Config)) *core.App {
	b.Helper()

	// Start with a default config
	cfg := config.NewDefaultConfig()
	// Apply any modifications for the specific benchmark scenario
	for _, modifier := range cfgModifiers {
		modifier(cfg)
	}

	// Create a provider with the modified config
	provider := config.NewProvider(cfg)

	// Create a mock app
	app := &core.App{}
	app.SetConfigProvider(provider)

	// Use a logger that discards output to avoid polluting benchmark results
	app.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Use a fresh, isolated cache for each benchmark
	c, err := cache.New[any]("small")
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}
	app.SetCache(c)

	return app
}

// monotonicIP generates a unique, ascending 4-byte IP address for a given integer i.
// This is used in benchmarks to ensure that each request comes from a unique source,
// preventing rate-limiting or blocking logic from contaminating the results of
// "happy path" tests. It uses the standard library to convert a uint32 directly
// into an IP string, starting from 0.0.0.0.
func monotonicIP(i int) string {
	ipBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(ipBytes, uint32(i))
	return net.IP(ipBytes).String()
}
