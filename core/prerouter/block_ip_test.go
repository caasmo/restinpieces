package prerouter

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/topk"
)

// TestBlockIP_GetClientIP verifies the IP extraction logic from a request.
func TestBlockIP_GetClientIP(t *testing.T) {
	testCases := []struct {
		name       string
		remoteAddr string
		expectedIP string
	}{
		{"IPv4 with port", "192.0.2.1:12345", "192.0.2.1"},
		{"IPv4 without port", "192.0.2.1", "192.0.2.1"},
		{"IPv6 with port", "[2001:db8::1]:12345", "2001:db8::1"},
		{"IPv6 without port", "2001:db8::1", "2001:db8::1"},
		{"Invalid address", "invalid-address", "invalid-address"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if got := GetClientIP(req); got != tc.expectedIP {
				t.Errorf("expected IP '%s', but got '%s'", tc.expectedIP, got)
			}
		})
	}
}

// TestBlockIP_WhenIPIsAlreadyBlocked verifies that a pre-blocked IP is rejected.
func TestBlockIP_WhenIPIsAlreadyBlocked(t *testing.T) {
	// --- Setup ---
	mockApp := &core.App{}
	rCache, _ := ristretto.New[any]()
	mockApp.SetCache(rCache)
	mockApp.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Manually block an IP by adding it to the cache.
	blockedIP := "192.0.2.100"
	now := time.Now()
	bucket := getTimeBucket(now)
	key := formatBlockKey(blockedIP, bucket)
	mockApp.Cache().Set(key, true, 1)
	// Wait for set to take effect
	time.Sleep(10 * time.Millisecond)

	cfg := config.NewDefaultConfig()
	provider := config.NewProvider(cfg)
	mockApp.SetConfigProvider(provider)

	middleware := NewBlockIp(mockApp)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler was called unexpectedly for a blocked IP")
	})
	handlerChain := middleware.Execute(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = blockedIP + ":12345"
	rr := httptest.NewRecorder()

	// --- Execution ---
	handlerChain.ServeHTTP(rr, req)

	// --- Verification ---
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected status code %d, but got %d", http.StatusTooManyRequests, rr.Code)
	}
}

// TestBlockIP_WhenIPIsNotBlocked verifies a non-blocked IP is allowed to pass.
func TestBlockIP_WhenIPIsNotBlocked(t *testing.T) {
	// --- Setup ---
	mockApp := &core.App{}
	rCache, _ := ristretto.New[any]()
	mockApp.SetCache(rCache)
	mockApp.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	cfg := config.NewDefaultConfig()
	provider := config.NewProvider(cfg)
	mockApp.SetConfigProvider(provider)

	middleware := NewBlockIp(mockApp)

	handlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})
	handlerChain := middleware.Execute(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:12345"
	rr := httptest.NewRecorder()

	// --- Execution ---
	handlerChain.ServeHTTP(rr, req)

	// --- Verification ---
	if rr.Code != http.StatusOK {
		t.Errorf("expected status code %d, but got %d", http.StatusOK, rr.Code)
	}
	if !handlerCalled {
		t.Error("next handler was not called for an allowed IP")
	}
}

// TestBlockIP_ProcessAndBlockTrigger verifies an IP is blocked after exceeding the threshold.
func TestBlockIP_ProcessAndBlockTrigger(t *testing.T) {
	// --- Setup ---
	mockApp := &core.App{}
	rCache, _ := ristretto.New[any]()
	mockApp.SetCache(rCache)
	mockApp.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// This test requires a custom-built middleware instance with highly sensitive
	// sketch parameters to ensure a block can be triggered reliably and quickly.
	sensitiveParams := topk.SketchParams{
		K: 2, WindowSize: 1, TickSize: 3, Width: 256, Depth: 2, ActivationRPS: 1, MaxSharePercent: 50,
	}
	// This configuration means: a tick is 3 requests. The window is 1 tick.
	// MaxShare is 50%. So, if 2 out of 3 requests are from the same IP, it will be blocked.
	sketch := topk.New(sensitiveParams)
	middleware := &BlockIp{
		app:    mockApp,
		sketch: sketch,
	}

	ipToBlock := "192.0.2.200"

	// --- Execution ---
	// Process IPs to trigger the blocking condition.
	_ = middleware.Process(ipToBlock)
	_ = middleware.Process("192.0.2.99") // A different IP to fill the tick.
	_ = middleware.Process(ipToBlock)    // This third request should trigger the block.

	// --- Verification ---
	// Because blocking is asynchronous (in a goroutine), we must poll the cache.
	var isBlocked bool
	for i := 0; i < 20; i++ { // Poll for up to 200ms.
		if middleware.IsBlocked(ipToBlock) {
			isBlocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !isBlocked {
		t.Fatal("IP was not blocked within the timeout period")
	}
}
