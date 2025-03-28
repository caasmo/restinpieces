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
	cs := &ConcurrentSketch{
		sketch:   instance,
		tickSize: tickSize,
	}
	slog.Debug("Initialized ConcurrentSketch",
		"tickSize", tickSize,
		"windowSize", instance.WindowSize,
		"threshold", cs.Threshold())
	return cs
}

// Incr wraps the sketch's Incr method with a mutex.
// Assuming sketch has an Incr method as described in the prompt.
func (cs *ConcurrentSketch) Incr(item string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.sketch.Incr(item)
}

// Count returns the count for an item in the sketch.
func (cs *ConcurrentSketch) Count(item string) uint32 {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	count := cs.sketch.Count(item)
	return count
}

// SortedSlice gets the sorted items and their counts from the sketch.
func (cs *ConcurrentSketch) TopK() []struct {
	Item  string
	Count uint32
} {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Get the sorted slice from the sketch
	items := cs.sketch.SortedSlice()
	slog.Debug("Yop IPs dump", "ips", items)

	// Convert to anonymous struct slice
	results := make([]struct {
		Item  string
		Count uint32
	}, len(items))

	for i, ic := range items {
		results[i] = struct {
			Item  string
			Count uint32
		}{
			Item:  ic.Item,
			Count: ic.Count,
		}
	}

	return results
}

// Tick wraps the sketch's Tick method with a mutex and increments the tick counter.
func (cs *ConcurrentSketch) Tick() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.sketch.Tick()
	cs.tickCount.Add(1)
}

// SizeBytes returns the memory usage of the underlying sketch in bytes
func (cs *ConcurrentSketch) SizeBytes() uint64 {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return uint64(cs.sketch.SizeBytes())
}

// Threshold returns thresholdPercent of the window capacity (WindowSize * tickSize)
// TODO we need time reference reference for low request/hour, maybe add time duration to struct
func (cs *ConcurrentSketch) Threshold() int {
	windowCapacity := uint64(cs.sketch.WindowSize) * cs.tickSize
	return int((windowCapacity * thresholdPercent) / 100)
}

// processTick handles the sketch tick and IP blocking logic
func (cs *ConcurrentSketch) processTick(a *App) {
	tickReqs := cs.TickReqCount.Add(1)
	_ = cs.Incr(ip)
	// Perform sketch operations
	if tickReqs >= cs.tickSize {
		cs.Tick() // Advance the sliding window
		tickReqs := cs.TickReqCount.Load()
		threshold := cs.Threshold()

		// Get top IPs from the sketch
		sortedIPs := cs.TopK()

		// Check IPs against the dynamic threshold
		for _, item := range sortedIPs {
			if item.Count > uint32(threshold) {
				if err := a.BlockIP(item.Item); err != nil {
					slog.Error("failed to block IP", "ip", item.Item, "error", err)
				}
			} else {
				// Since the list is sorted, we can break early
				break
			}
		}

		// Reset counter atomically
		if cs.TickReqCount.CompareAndSwap(tickReqs, 0) {
			// TODO
		}
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
				return
			}

			cs.processTick(a)

			// Proceed to the next handler in the chain
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
