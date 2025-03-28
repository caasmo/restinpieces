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
type ConcurrentSketch struct {
	mu            sync.Mutex
	sketch        *sliding.Sketch
	tickSize      uint64        // Number of requests before processing the sketch
	blockThreshold uint32       // Threshold above which IPs are flagged for blocking
	totalReqs     atomic.Uint64 // Counter for total requests processed since last tick
}

// NewConcurrentSketch creates a new thread-safe sketch wrapper.
// tickSize: How many requests trigger a sketch tick and top-k check.
// blockThreshold: The count above which an IP is flagged for blocking.
func NewConcurrentSketch(instance *sliding.Sketch, tickSize uint64, blockThreshold uint32) *ConcurrentSketch {
	if instance == nil {
		// Handle nil sketch instance appropriately, maybe return error or panic
		panic("sketch instance cannot be nil for ConcurrentSketch")
	}
	if tickSize == 0 {
		tickSize = 1000 // Default tick size if not specified
	}
	cs := &ConcurrentSketch{
		sketch:        instance,
		tickSize:      tickSize,
		blockThreshold: blockThreshold,
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

// SortedSlice gets the sorted items and their counts from the sketch.
func (cs *ConcurrentSketch) SortedSlice() []struct {
	Item  string
	Count uint32
} {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	// Get the sorted slice from the sketch
	itemCounts := cs.sketch.SortedSlice()
	
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
func NewBlockMiddlewareFunc(concurrentSketch *ConcurrentSketch) func(http.Handler) http.Handler {
	if concurrentSketch == nil {
		// Initialize the underlying sketch
		sketch := sliding.New(3, 60, sliding.WithWidth(1024), sliding.WithDepth(3))
		log.Println("the sketch takes up", sketch.SizeBytes(), "bytes in memory")
		
		// Create a new ConcurrentSketch with default tick size and block threshold
		concurrentSketch = NewConcurrentSketch(sketch, 1000, 100) // Default values for tickSize and blockThreshold
	}

	// Directly return the handler function
	return func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

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

					// Get sorted IPs from the sketch
					sortedIPs := concurrentSketch.SortedSlice()

					// Check IPs against the threshold
					for _, item := range sortedIPs {
						if item.Count > concurrentSketch.blockThreshold {
							// Log that this IP should be blocked
							slog.Warn("IP exceeded threshold, should be blocked", "ip", item.Item, "count", item.Count, "threshold", concurrentSketch.blockThreshold)
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
