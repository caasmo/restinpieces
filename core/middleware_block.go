package core

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	// Placeholder for the actual sketch library import
	sketch "github.com/your/sketch/library" // Placeholder for the actual sketch library import
)

// ConcurrentSketch provides thread-safe access to a sketch instance and manages ticking.
type ConcurrentSketch struct {
	mu        sync.Mutex
	sketch    *sketch.Sketch
	tickSize  uint64        // Number of requests before processing the sketch
	totalReqs atomic.Uint64 // Counter for total requests processed since last tick
}

// NewConcurrentSketch creates a new thread-safe sketch wrapper.
// tickSize: How many requests trigger a sketch tick and top-k check.
func NewConcurrentSketch(instance *sketch.Sketch, tickSize uint64) *ConcurrentSketch {
	if instance == nil {
		// Handle nil sketch instance appropriately, maybe return error or panic
		panic("sketch instance cannot be nil for ConcurrentSketch")
	}
	if tickSize == 0 {
		tickSize = 1000 // Default tick size if not specified
	}
	cs := &ConcurrentSketch{
		sketch:   instance,
		tickSize: tickSize,
	}
	// cs.totalReqs is initialized to 0 by default via atomic.Uint64
	return cs
}

// Add wraps the sketch's Add method with a mutex.
func (cs *ConcurrentSketch) Add(item string, increment uint32) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.sketch.Add(item, increment)
}

// Count wraps the sketch's Count method with a mutex.
// Assuming sketch has a Count method as described in the prompt.
func (cs *ConcurrentSketch) Count(item string) uint32 {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.sketch.Count(item)
}

// Incr wraps the sketch's Incr method with a mutex.
// Assuming sketch has an Incr method as described in the prompt.
func (cs *ConcurrentSketch) Incr(item string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.sketch.Incr(item)
}

// Tick wraps the sketch's Tick method with a mutex.
func (cs *ConcurrentSketch) Tick() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.sketch.Tick()
}

// SortedSlice wraps the sketch's SortedSlice method with a mutex.
// Assuming sketch has a SortedSlice method returning []sketch.ItemCount
func (cs *ConcurrentSketch) SortedSlice() []sketch.ItemCount {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.sketch.SortedSlice()
}

// --- IP Blocking Middleware Function ---

// NewBlockMiddlewareFunc creates a middleware function that uses a ConcurrentSketch
// to identify and potentially block IPs based on request frequency.
// blockThreshold: The count above which an IP is flagged for blocking.
// concurrentSketch: A pre-initialized ConcurrentSketch instance.
func NewBlockMiddlewareFunc(blockThreshold uint32, concurrentSketch *ConcurrentSketch) func(http.Handler) http.Handler {
	if concurrentSketch == nil {
		panic("concurrentSketch cannot be nil for middleware")
	}

	// The actual middleware function returned
	return func(next http.Handler) http.Handler {

		// The handler function that processes each request
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Extract client IP
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				// Handle error potentially, or use RemoteAddr directly if no port
				ip = r.RemoteAddr
				// Consider X-Forwarded-For header if behind a proxy
				if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
					// Use the first IP in the list
					parts := strings.Split(forwarded, ",")
					ip = strings.TrimSpace(parts[0])
				}
			}

			// TODO: Check if IP is already in the blocklist before processing
			// if bm.isBlocked(ip) {
			// 	http.Error(w, "Forbidden", http.StatusForbidden)
			// 	return
			// }

			// TODO: Check if IP is already in the blocklist before processing
			// if concurrentSketch.isBlocked(ip) { // Assuming blocklist is managed within ConcurrentSketch or elsewhere
			// 	http.Error(w, "Forbidden", http.StatusForbidden)
			// 	return
			// }

			// Increment total request count atomically within the sketch wrapper
			currentTotal := concurrentSketch.totalReqs.Add(1)

			// Add IP to the concurrent sketch
			concurrentSketch.Add(ip, 1)

			// Check if it's time to tick and check top-k
			if currentTotal >= concurrentSketch.tickSize {
				// Reset counter atomically - only one goroutine should perform the tick logic.
				// Using CompareAndSwap to ensure only the goroutine that reaches the threshold performs the tick.
				if concurrentSketch.totalReqs.CompareAndSwap(currentTotal, 0) {

					// Perform sketch operations using the thread-safe wrapper
					concurrentSketch.Tick() // Advance the sliding window

					// Get top K IPs from the sketch
					topK := concurrentSketch.SortedSlice()

					// Check top K IPs against the threshold
					for _, item := range topK {
						if item.Count > blockThreshold {
							// Log that this IP should be blocked
							slog.Warn("IP exceeded threshold, should be blocked", "ip", item.Item, "count", item.Count, "threshold", blockThreshold)
							// TODO: Add IP to the actual blocklist here
							// concurrentSketch.blockIP(item.Item) // Assuming blocklist is managed within ConcurrentSketch or elsewhere
						} else {
							// Since the list is sorted, we can potentially break early
							// if counts are guaranteed to be non-increasing.
							break
						}
					}
					// No unlock needed here as wrapper methods handle locking
				}
			}

			// Proceed to the next handler in the chain
			next.ServeHTTP(w, r)
		}

		// Return the handler function wrapped in http.HandlerFunc
		return http.HandlerFunc(fn)
	}
}

// TODO: Decide where to implement and manage the actual blocklist (e.g., within ConcurrentSketch or a separate service)
// Example potential methods for ConcurrentSketch if blocklist is managed there:
// func (cs *ConcurrentSketch) blockIP(ip string) { ... }
// func (cs *ConcurrentSketch) isBlocked(ip string) bool { ... }
