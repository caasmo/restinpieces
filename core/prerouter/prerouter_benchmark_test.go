package prerouter

import (
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/router"
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
	cache, err := ristretto.New[any]("small")
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}
	app.SetCache(cache)

	return app
}

// --- Suite 1: Individual Middleware Benchmarks ---

// BenchmarkRecorder measures the overhead of the Recorder middleware.
func BenchmarkRecorder(b *testing.B) {
	app := newBenchmarkApp(b)
	middleware := NewRecorder(app)
	handler := middleware.Execute(noOpHandler)
	req := httptest.NewRequest("GET", "/", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

// BenchmarkRequestLog_Active measures the overhead of the RequestLog middleware when active.
func BenchmarkRequestLog_Active(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.Log.Request.Activated = true
	})
	// RequestLog depends on Recorder
	middleware := NewRecorder(app).Execute(NewRequestLog(app).Execute(noOpHandler))
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:12345"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
	}
}

// BenchmarkRequestLog_Inactive measures the overhead of the RequestLog middleware when inactive.
func BenchmarkRequestLog_Inactive(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.Log.Request.Activated = false
	})
	middleware := NewRequestLog(app).Execute(noOpHandler)
	req := httptest.NewRequest("GET", "/", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
	}
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

// BenchmarkBlockIp_Process measures the "happy path" for BlockIp: a new IP is processed by the sketch.
// It generates a unique IP for each request to ensure the sketch never triggers a block,
// thus providing a pure measurement of the process path's runtime performance.
func BenchmarkBlockIp_Process(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockIp.Enabled = true
		cfg.BlockIp.Activated = true
		// The default config uses "medium" level.
		// Based on core/prerouter/block_ip.go, "medium" settings are:
		// - WindowSize: 10, TickSize: 100 -> 1000 request window
		// - MaxSharePercent: 35 -> 350 request threshold
		// To prevent any IP from being blocked, we must use a unique IP for each request.
	})
	// Create the middleware once, as it would be in a real server.
	middleware := NewBlockIp(app).Execute(noOpHandler)

	// Pre-generate a slice of requests with unique IP addresses.
	// This avoids repeated work inside the loop and ensures we only measure the handler.
	reqs := make([]*http.Request, b.N)
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		// Generate a unique IP for every single request to guarantee the blocking
		// threshold is never met. This provides a pure test of the process path.
		req.RemoteAddr = monotonicIP(i) + ":12345"
		reqs[i] = req
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		middleware.ServeHTTP(httptest.NewRecorder(), reqs[i])
	}
}

// BenchmarkBlockIp_Blocked measures the cost of rejecting an already-blocked IP.
func BenchmarkBlockIp_Blocked(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockIp.Enabled = true
		cfg.BlockIp.Activated = true
	})
	middleware := NewBlockIp(app)
	handler := middleware.Execute(noOpHandler)

	// Pre-block the IP
	blockedIP := "192.0.2.100"
	if err := middleware.Block(blockedIP); err != nil {
		b.Fatalf("Failed to block IP: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = blockedIP + ":12345"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// BenchmarkBlockUaList_NoMatch measures the cost of a UA check that doesn't match.
func BenchmarkBlockUaList_NoMatch(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockUaList.Activated = true
		cfg.BlockUaList.List.Regexp = regexp.MustCompile(`^BadBot/.*`)
	})
	middleware := NewBlockUaList(app).Execute(noOpHandler)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "GoodBot/1.0")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		middleware.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// BenchmarkBlockUaList_Match measures the cost of a UA check that matches and blocks.
func BenchmarkBlockUaList_Match(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockUaList.Activated = true
		cfg.BlockUaList.List.Regexp = regexp.MustCompile(`^BadBot/.*`)
	})
	middleware := NewBlockUaList(app).Execute(noOpHandler)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "BadBot/2.0")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		middleware.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// BenchmarkBlockHost_Allowed measures an allowed host check.
func BenchmarkBlockHost_Allowed(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockHost.Activated = true
		cfg.BlockHost.AllowedHosts = []string{"example.com", "*.example.org"}
	})
	middleware := NewBlockHost(app).Execute(noOpHandler)
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		middleware.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// BenchmarkBlockHost_Blocked measures a blocked host check.
func BenchmarkBlockHost_Blocked(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockHost.Activated = true
		cfg.BlockHost.AllowedHosts = []string{"example.com", "*.example.org"}
	})
	middleware := NewBlockHost(app).Execute(noOpHandler)
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "blocked.com"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		middleware.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// BenchmarkBlockRequestBody_Allowed measures allowing a request with a small body.
func BenchmarkBlockRequestBody_Allowed(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockRequestBody.Activated = true
		cfg.BlockRequestBody.Limit = 1024
	})
	// The handler must read the body to test the middleware
	readingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
	})
	middleware := NewBlockRequestBody(app).Execute(readingHandler)
	body := strings.NewReader(strings.Repeat("a", 512))
	req := httptest.NewRequest("POST", "/", body)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// We must reset the body for each iteration
		body.Seek(0, io.SeekStart)
		middleware.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// BenchmarkBlockRequestBody_Blocked measures blocking a request with a large body.
func BenchmarkBlockRequestBody_Blocked(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockRequestBody.Activated = true
		cfg.BlockRequestBody.Limit = 1024
	})
	// The handler must read the body to trigger the block
	readingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		// In a real scenario, MaxBytesReader writes the error. Here we simulate it.
		if r.ContentLength > app.Config().BlockRequestBody.Limit {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
		}
	})
	middleware := NewBlockRequestBody(app).Execute(readingHandler)
	body := strings.NewReader(strings.Repeat("a", 2048))
	req := httptest.NewRequest("POST", "/", body)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		body.Seek(0, io.SeekStart)
		middleware.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// --- Suite 2: Realistic End-to-End Scenarios ---

// buildFullChain constructs the entire prerouter middleware chain for benchmarking,
// mimicking the execution order in restinpieces.go.
func buildFullChain(app *core.App) http.Handler {
	preRouterChain := router.NewChain(noOpHandler)
	cfg := app.Config()

	// Middlewares are added in the order of execution, matching the logic
	// in router.WithMiddleware where the first middleware added is the first to execute.
	// Execution Order: Recorder -> RequestLog -> BlockIp -> Metrics -> ...

	// 1. Recorder
	preRouterChain.WithMiddleware(NewRecorder(app).Execute)

	// 2. RequestLog
	preRouterChain.WithMiddleware(NewRequestLog(app).Execute)

	// 3. BlockIp
	if cfg.BlockIp.Enabled {
		preRouterChain.WithMiddleware(NewBlockIp(app).Execute)
	}

	// 4. Metrics
	if cfg.Metrics.Enabled {
		testMetrics, _ := newTestMetricsMiddleware(app)
		preRouterChain.WithMiddleware(testMetrics.Execute)
	}

	// 5. BlockUaList
	preRouterChain.WithMiddleware(NewBlockUaList(app).Execute)

	// 6. BlockHost
	preRouterChain.WithMiddleware(NewBlockHost(app).Execute)

	// 7. TLSHeaderSTS
	preRouterChain.WithMiddleware(NewTLSHeaderSTS().Execute)

	// 8. Maintenance
	preRouterChain.WithMiddleware(NewMaintenance(app).Execute)

	// 9. BlockRequestBody
	preRouterChain.WithMiddleware(NewBlockRequestBody(app).Execute)

	return preRouterChain.Handler()
}

// BenchmarkChain_HappyPath measures the full chain with a valid request.
func BenchmarkChain_HappyPath(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.Log.Request.Activated = true
		cfg.BlockIp.Enabled = true
		cfg.BlockIp.Activated = true
		cfg.Metrics.Enabled = true
		cfg.Metrics.Activated = true
		cfg.BlockUaList.Activated = true
		cfg.BlockUaList.List.Regexp = regexp.MustCompile(`^BadBot/.*`)
		cfg.BlockHost.Activated = true
		cfg.BlockHost.AllowedHosts = []string{"example.com"}
		cfg.Maintenance.Activated = false
	})
	handler := buildFullChain(app)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:12345"
	req.Host = "example.com"
	req.Header.Set("User-Agent", "GoodBot/1.0")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// BenchmarkChain_Blocked_Maintenance measures an early exit due to maintenance mode.
func BenchmarkChain_Blocked_Maintenance(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		// Only maintenance mode needs to be active for this test
		cfg.Maintenance.Activated = true
	})
	handler := buildFullChain(app)
	req := httptest.NewRequest("GET", "/", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// BenchmarkChain_Blocked_Host measures an early exit due to a blocked host.
func BenchmarkChain_Blocked_Host(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockHost.Activated = true
		cfg.BlockHost.AllowedHosts = []string{"example.com"}
	})
	handler := buildFullChain(app)
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "blocked.com"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
}
