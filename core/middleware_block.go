package core

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/keilerkonzept/topk/sliding"
)

// ConcurrentSketch provides thread-safe access to a sketch instance and manages ticking.
const (
	thresholdPercent = 10 // 10% of window capacity
)

type ConcurrentSketch struct {
	mu            sync.Mutex
	sketch        *sliding.Sketch
	tickSize      uint64        // number of request per tick 
	totalReqs     atomic.Uint64 // Counter for total requests processed since last tick
	tickCount     atomic.Uint64 // Counter for total ticks processed
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
		"windowSize", instance.WindowSize)
	return cs
}


// Incr wraps the sketch's Incr method with a mutex.
// Assuming sketch has an Incr method as described in the prompt.
func (cs *ConcurrentSketch) Incr(item string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.sketch.Incr(item)
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
func (cs *ConcurrentSketch) Threshold() int {
	windowCapacity := uint64(cs.sketch.WindowSize) * cs.tickSize
	return int((windowCapacity * thresholdPercent) / 100)
}

// processTick checks for IPs exceeding the threshold and logs them
func (cs *ConcurrentSketch) processTick() {
	// Perform sketch operations using the thread-safe wrapper
	cs.Tick() // Advance the sliding window
	tickNum := cs.tickCount.Load()
	threshold := cs.Threshold()
	slog.Debug("TICK:", 
		"number", tickNum, 
		"currentTotal", cs.totalReqs.Load(),
		"sizeBytes", cs.SizeBytes(),
		"threshold", threshold)

	// Get sorted IPs from the sketch
	sortedIPs := cs.SortedSlice()

	// Check IPs against the dynamic threshold
	for _, item := range sortedIPs {
		if item.Count > uint32(threshold) {
			slog.Warn("IP exceeded threshold, should be blocked", 
				"ip", item.Item, 
				"count", item.Count, 
				"threshold", threshold)
			// TODO: Add IP to the actual blocklist here
		} else {
			// Since the list is sorted, we can break early
			break
		}
	}
}

// Count returns the count for an item in the sketch.
func (cs *ConcurrentSketch) Count(item string) uint32 {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	count := cs.sketch.Count(item)
	return count
}

// SortedSlice gets the sorted items and their counts from the sketch.
func (cs *ConcurrentSketch) SortedSlice() []struct {
	Item  string
	Count uint32
} {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	// Get the sorted slice from the sketch
	itemCounts := cs.sketch.SortedSlice()
    slog.Debug("Sorted IPs dump", "ips", itemCounts)
	
	// Convert to anonymous struct slice
	results := make([]struct {
		Item  string
		Count uint32
	}, len(itemCounts))
	
	for i, ic := range itemCounts {
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

// --- IP Blocking Middleware Function ---

// NewBlockMiddlewareFunc creates a middleware function that uses a ConcurrentSketch
// to identify and potentially block IPs based on request frequency.
// concurrentSketch: A pre-initialized ConcurrentSketch instance.
func NewBlockMiddlewareFunc(cs *ConcurrentSketch) func(http.Handler) http.Handler {
	if cs == nil {
		// Initialize the underlying sketch
		//sketch := sliding.New(3, 60, sliding.WithWidth(1024), sliding.WithDepth(3))
		sketch := sliding.New(3, 10, sliding.WithWidth(1024), sliding.WithDepth(3))
		//sketch := sliding.New(3, 10, sliding.WithWidth(1024), sliding.WithDepth(3))
		slog.Info("sketch memory usage", "bytes", sketch.SizeBytes())
		
		// Create a new ConcurrentSketch with default tick size
		cs = NewConcurrentSketch(sketch, 100) // Default tickSize
	}

	// Return the middleware function
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			
			// Debug log incoming request
			slog.Debug("-------------------------------------------------------------------", 
				"ip", ip, 
				"method", r.Method, 
				"path", r.URL.Path)

			// Increment total request count atomically within the sketch wrapper
			currentTotal := cs.totalReqs.Add(1)
			slog.Debug("incremented request count", "total", currentTotal)

			// Increment IP count in the sketch
			_ = cs.Incr(ip)
			slog.Debug("incremented IP in sketch", "ip", ip, "count", cs.Count(ip))

			// Check if it's time to tick and check top-k
			if currentTotal >= cs.tickSize {
				// Reset counter atomically - only one goroutine should perform the tick logic.
				// Using CompareAndSwap to ensure only the goroutine that reaches the threshold performs the tick.
				cs.processTick()
				if cs.totalReqs.CompareAndSwap(currentTotal, 0) {
                    // TODO

				}
			}

			// Proceed to the next handler in the chain
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// getClientIP extracts the client IP address from the request, handling proxies via X-Forwarded-For header
func getClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Handle error potentially, or use RemoteAddr directly if no port
		ip = r.RemoteAddr
	}
	// Consider X-Forwarded-For header if behind a proxy
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Use the first IP in the list
		parts := strings.Split(forwarded, ",")
		ip = strings.TrimSpace(parts[0])
	}
	return ip
}

// TODO: Decide where to implement and manage the actual blocklist (e.g., within ConcurrentSketch or a separate service)
// Example potential methods for ConcurrentSketch if blocklist is managed there:
// func (cs *ConcurrentSketch) blockIP(ip string) { ... }
// func (cs *ConcurrentSketch) isBlocked(ip string) bool { ... }
