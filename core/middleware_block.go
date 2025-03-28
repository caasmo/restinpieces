package core

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	// Placeholder for the actual sketch library import
	sketch "github.com/your/sketch/library"
)

// BlockMiddleware uses a sliding-window top-k sketch to identify and potentially block IPs.
type BlockMiddleware struct {
	mu             sync.Mutex
	sketch         *sketch.Sketch // The underlying sketch instance
	tickSize       uint64         // Number of requests before processing the sketch
	totalReqs      atomic.Uint64  // Counter for total requests processed since last tick
	blockThreshold uint32         // Request count threshold for blocking an IP within a window
	next           http.Handler   // The next handler in the chain
	// TODO: Add a blocklist map/set here later
}

// NewBlockMiddleware creates and initializes a new BlockMiddleware.
// tickSize: How many requests trigger a sketch tick and top-k check.
// blockThreshold: The count above which an IP is flagged for blocking.
// sketchInstance: A pre-initialized sketch instance.
// next: The next http.Handler.
func NewBlockMiddleware(tickSize uint64, blockThreshold uint32, sketchInstance *sketch.Sketch, next http.Handler) (*BlockMiddleware, error) {
	// Basic validation
	if tickSize == 0 {
		tickSize = 1000 // Default tick size
	}
	if sketchInstance == nil {
		// In a real scenario, you might initialize the sketch here if not provided
		// For now, we assume it's provided or handle the error appropriately.
		// Example: sketchInstance, err = sketch.New(...)
		// if err != nil { return nil, err }
		panic("sketchInstance cannot be nil") // Or return an error
	}

	bm := &BlockMiddleware{
		sketch:         sketchInstance,
		tickSize:       tickSize,
		blockThreshold: blockThreshold,
		next:           next,
	}
	// bm.totalReqs is initialized to 0 by default via atomic.Uint64

	return bm, nil
}

// ServeHTTP implements the http.Handler interface.
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

	// Add IP to sketch (thread-safe within the sketch assumed, or lock if needed)
	// Using Add(item, 1) as per the prompt's description.
	// The sketch implementation needs to handle concurrency internally or we lock here.
	// Assuming sketch methods are NOT internally thread-safe, we lock.
	bm.mu.Lock()
	bm.sketch.Add(ip, 1) // Add or increment the count for this IP
	bm.mu.Unlock()

	// Check if it's time to tick and check top-k
	if currentTotal >= bm.tickSize {
		// Reset counter atomically - only one goroutine should perform the tick logic.
		// Using CompareAndSwap to ensure only the goroutine that reaches the threshold performs the tick.
		if bm.totalReqs.CompareAndSwap(currentTotal, 0) {
			bm.mu.Lock()
			bm.sketch.Tick() // Advance the sliding window

			// Get top K IPs from the sketch
			// Assuming SortedSlice returns []sketch.ItemCount{Item string, Count uint32}
			topK := bm.sketch.SortedSlice()

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
			bm.mu.Unlock()
		}
	}

	// Proceed to the next handler
	bm.next.ServeHTTP(w, r)
}

// TODO: Implement isBlocked(ip string) bool
// TODO: Implement blockIP(ip string)

