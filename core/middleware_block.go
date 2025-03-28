package core

import (
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/keilerkonzept/topk/sliding"
)

// ConcurrentSketch provides thread-safe access to a sketch instance and manages ticking.
const (
	thresholdPercent = 80 // 10% of window capacity
)

type ConcurrentSketch struct {
	mu           sync.Mutex
	sketch       *sliding.Sketch
	tickSize     uint64        // number of request per tick
	TickReqCount atomic.Uint64 // Counter for requests processed since last tick
	tickCount    atomic.Uint64 // Counter for total ticks processed
}

// NewConcurrentSketch creates a new thread-safe sketch wrapper.
// tickSize: How many requests trigger a sketch tick and top-k check.
func NewConcurrentSketch(instance *sliding.Sketch, tickSize uint64) *ConcurrentSketch {
	if instance == nil {
		panic("sketch instance cannot be nil for ConcurrentSketch")
	}
	if tickSize == 0 {
		tickSize = 1000 // Default tick size if not specified
	}
	return &ConcurrentSketch{
		sketch:   instance,
		tickSize: tickSize,
	}
}

// processTick handles the sketch tick and IP blocking logic
func (cs *ConcurrentSketch) processTick(a *App, ip string) {
	tickReqs := cs.TickReqCount.Add(1)
	
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	cs.sketch.Incr(ip)
	
	if tickReqs >= cs.tickSize {
		cs.sketch.Tick()
		cs.tickCount.Add(1)
		cs.TickReqCount.Store(0)
	}
}

// --- IP Blocking Middleware Function ---

// BlockMiddleware creates a middleware function that uses a ConcurrentSketch
// to identify and potentially block IPs based on request frequency.
func (a *App) BlockMiddleware() func(http.Handler) http.Handler {
	// TODO
	// Initialize the underlying sketch
	sketch := sliding.New(3, 10, sliding.WithWidth(1024), sliding.WithDepth(3))
	slog.Info("sketch memory usage", "bytes", sketch.SizeBytes())

	// Create a new ConcurrentSketch with default tick size
	cs := NewConcurrentSketch(sketch, 100) // Default tickSize

	// Return the middleware function
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ip := a.GetClientIP(r)

			// Check if IP is already blocked

			// TODO not here
			if a.IsBlocked(ip) {
				writeJsonError(w, errorIpBlocked)
				slog.Info("IP blocked from accessing endpoint", "ip", ip)
				return
			}

			cs.processTick(a, ip)

			// Proceed to the next handler in the chain
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
