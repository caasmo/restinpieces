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

// ConcurrentSketch provides thread-safe access to a sketch instance.
type ConcurrentSketch struct {
	mu     sync.Mutex
	sketch *sketch.Sketch
}

// NewConcurrentSketch creates a new thread-safe sketch wrapper.
func NewConcurrentSketch(instance *sketch.Sketch) *ConcurrentSketch {
	if instance == nil {
		// Handle nil sketch instance appropriately, maybe return error or panic
		panic("sketch instance cannot be nil for ConcurrentSketch")
	}
	return &ConcurrentSketch{
		sketch: instance,
	}
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

// --- Block Middleware ---

// BlockMiddleware uses a ConcurrentSketch to identify and potentially block IPs based on request frequency.
type BlockMiddleware struct {
	concurrentSketch *ConcurrentSketch // Thread-safe sketch wrapper
	tickSize         uint64            // Number of requests before processing the sketch
	totalReqs        atomic.Uint64     // Counter for total requests processed since last tick
	blockThreshold   uint32            // Request count threshold for blocking an IP within a window
	next             http.Handler      // The next handler in the chain
	// TODO: Add a blocklist map/set here later (needs concurrent access, e.g., sync.Map or mutex)
}

// NewBlockMiddleware creates and initializes a new BlockMiddleware.
// tickSize: How many requests trigger a sketch tick and top-k check.
// blockThreshold: The count above which an IP is flagged for blocking.
// concurrentSketch: A pre-initialized ConcurrentSketch instance.
// next: The next http.Handler.
func NewBlockMiddleware(tickSize uint64, blockThreshold uint32, concurrentSketch *ConcurrentSketch, next http.Handler) (*BlockMiddleware, error) {
	// Basic validation
	if tickSize == 0 {
		tickSize = 1000 // Default tick size
	}
	if concurrentSketch == nil {
		panic("concurrentSketch cannot be nil") // Or return an error
	}
	if next == nil {
		panic("next handler cannot be nil") // Or return an error
	}

	bm := &BlockMiddleware{
		concurrentSketch: concurrentSketch,
		tickSize:         tickSize,
		blockThreshold:   blockThreshold,
		next:             next,
	}
	// bm.totalReqs is initialized to 0 by default via atomic.Uint64

	return bm, nil
}

// ServeHTTP implements the http.Handler interface for the blocking middleware.
func (bm *BlockMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	// Increment total request count atomically
	currentTotal := bm.totalReqs.Add(1)

	// Add IP to the concurrent sketch
	bm.concurrentSketch.Add(ip, 1)

	// Check if it's time to tick and check top-k
	if currentTotal >= bm.tickSize {
		// Reset counter atomically - only one goroutine should perform the tick logic.
		// Using CompareAndSwap to ensure only the goroutine that reaches the threshold performs the tick.
		if bm.totalReqs.CompareAndSwap(currentTotal, 0) {

			// Perform sketch operations using the thread-safe wrapper
			bm.concurrentSketch.Tick() // Advance the sliding window

			// Get top K IPs from the sketch
			topK := bm.concurrentSketch.SortedSlice()

			// Check top K IPs against the threshold
			for _, item := range topK {
				if item.Count > bm.blockThreshold {
					// Log that this IP should be blocked
					slog.Warn("IP exceeded threshold, should be blocked", "ip", item.Item, "count", item.Count, "threshold", bm.blockThreshold)
					// TODO: Add IP to the actual blocklist here
					// bm.blockIP(item.Item)
				} else {
					// Since the list is sorted, we can potentially break early
					// if counts are guaranteed to be non-increasing.
					break
				}
			}
			// No unlock needed here as wrapper methods handle locking
		}
	}

	// Proceed to the next handler
	bm.next.ServeHTTP(w, r)
}

// TODO: Implement isBlocked(ip string) bool
// TODO: Implement blockIP(ip string)

