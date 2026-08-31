package prerouter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

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

// BenchmarkBlockOversizedRequest_Allowed measures allowing a request with a small body.
func BenchmarkBlockOversizedRequest_Allowed(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockOversizedRequest.Activated = true
		cfg.BlockOversizedRequest.BodyLimit = 1024
	})
	// The handler must read the body to test the middleware
	readingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
	})
	middleware := NewBlockOversizedRequest(app).Execute(readingHandler)
	body := strings.NewReader(strings.Repeat("a", 512))
	req := httptest.NewRequest("POST", "/", body)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// We must reset the body for each iteration
		if _, err := body.Seek(0, io.SeekStart); err != nil {
			b.Fatalf("Failed to seek body: %v", err)
		}
		middleware.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// BenchmarkBlockOversizedRequest_Blocked measures blocking a request with a large body.
func BenchmarkBlockOversizedRequest_Blocked(b *testing.B) {
	app := newBenchmarkApp(b, func(cfg *config.Config) {
		cfg.BlockOversizedRequest.Activated = true
		cfg.BlockOversizedRequest.BodyLimit = 1024
	})
	// The handler must read the body to trigger the block
	readingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		// In a real scenario, MaxBytesReader writes the error. Here we simulate it.
		if r.ContentLength > app.Config().BlockOversizedRequest.BodyLimit {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
		}
	})
	middleware := NewBlockOversizedRequest(app).Execute(readingHandler)
	body := strings.NewReader(strings.Repeat("a", 2048))
	req := httptest.NewRequest("POST", "/", body)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := body.Seek(0, io.SeekStart); err != nil {
			b.Fatalf("Failed to seek body: %v", err)
		}
		middleware.ServeHTTP(httptest.NewRecorder(), req)
	}
}
