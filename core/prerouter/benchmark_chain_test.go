package prerouter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/router"
)

// buildChain constructs the entire prerouter middleware chain for benchmarking,
// mimicking the execution order in restinpieces.go.
func buildChain(app *core.App) http.Handler {
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

	// 9. BlockOversizedRequest
	preRouterChain.WithMiddleware(NewBlockOversizedRequest(app).Execute)

	// 10. BlockEndpointsMismatch
	preRouterChain.WithMiddleware(NewBlockEndpointsMismatch(app).Execute)

	return preRouterChain.Handler()
}

// BenchmarkChain_HappyPath measures the full chain with a valid request.
// Realistic chain is just the default config with no changes. It uses a
// discard logger, so it times middleware only — the DB flush is timed
// separately by BenchmarkLog_InsertBatch.
func BenchmarkChain_HappyPath(b *testing.B) {
	app := newBenchmarkApp(b)
	handler := buildChain(app)

	// Use a single request object and modify its RemoteAddr in the loop.
	// This avoids a large upfront allocation and the high overhead of calling
	// b.StopTimer/b.StartTimer in a tight loop. The tiny, constant cost of
	// updating the IP is the most acceptable form of measurement noise.
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"
	req.Header.Set("User-Agent", "GoodBot/1.0")

	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		req.RemoteAddr = monotonicIP(i) + ":12345"
		handler.ServeHTTP(httptest.NewRecorder(), req)
		i++
	}
}

// BenchmarkChain_Blocked_Maintenance measures an early exit due to maintenance mode.
func BenchmarkChain_Blocked_Maintenance(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		// Enable all preceding middleware to create a realistic chain.
		cfg.BlockIp.Enabled = true
		cfg.BlockIp.Activated = true
		cfg.BlockHost.Activated = true
		cfg.BlockHost.AllowedHosts = []string{"example.com"}
		// Finally, activate maintenance mode, which is what we want to measure.
		cfg.Maintenance.Activated = true
	})
	handler := buildChain(app)

	// Use a single request object, modifying the IP inside the loop to ensure
	// no preceding middleware blocks the request before the maintenance check.
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "example.com"

	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		req.RemoteAddr = monotonicIP(i) + ":12345"
		handler.ServeHTTP(httptest.NewRecorder(), req)
		i++
	}
}

// BenchmarkChain_Blocked_Host measures an early exit due to a blocked host.
func BenchmarkChain_Blocked_Host(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockHost.Activated = true
		cfg.BlockHost.AllowedHosts = []string{"example.com"}
		// Also enable BlockIp so the chain is realistic.
		cfg.BlockIp.Enabled = true
		cfg.BlockIp.Activated = true
	})
	handler := buildChain(app)

	// Use a single request object, modifying the IP inside the loop to ensure
	// the IP blocker does not fire before the host blocker.
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "blocked.com"

	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		req.RemoteAddr = monotonicIP(i) + ":12345"
		handler.ServeHTTP(httptest.NewRecorder(), req)
		i++
	}
}
