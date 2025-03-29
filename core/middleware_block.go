package core

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/keilerkonzept/topk/sliding"
)

// ConcurrentSketch provides thread-safe access to a sketch instance and manages ticking.
const (
	thresholdPercent = 80 // 10% of window capacity
)

type ConcurrentSketch struct {
	mu        sync.Mutex
	sketch    *sliding.Sketch
	tickSize  uint64 // number of request per tick
	tickReq   uint64 // Counter for requests processed since last tick
	tickCount uint64 // Counter for total ticks processed
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

func (cs *ConcurrentSketch) processTick(a *App, ip string) {
    cs.mu.Lock()
    defer cs.mu.Unlock()

    cs.sketch.Incr(ip)
    cs.tickReq++

    if cs.tickReq >= cs.tickSize {
        cs.sketch.Tick()
        cs.tickCount++
        cs.tickReq = 0

        windowCapacity := uint64(cs.sketch.WindowSize) * cs.tickSize
        threshold := int((windowCapacity * thresholdPercent) / 100)

        items := cs.sketch.SortedSlice()

        // Phase 1: Collect IPs to block (under mutex protection)
        ipsToBlock := make([]string, 0)
        for _, item := range items {
            if item.Count > uint32(threshold) {
                ipsToBlock = append(ipsToBlock, item.Item)
            } else {
                break
            }
        }

        // Release the mutex before calling Ristretto
        //
        // Even if multiple goroutines call a.BlockIP for the same IP
        // concurrently, Ristretto will handle it safely. Blocking an IP
        // multiple times is harmless if the operation is idempotent (same key).
        // Ristretto batches writes into a ring buffer, so frequent Set calls
        // for the same key will be merged efficiently. The last write (in
        // buffer order) will determine the final value.
        // Ristretto uses a buffered write mechanism (a ring buffer) to batch
        // Set/Del operations for performance.
        go func(ips []string) {
            for _, ip := range ips {
                if err := a.BlockIP(ip); err != nil {
                    // Handle error (e.g., log it)
                }
            }
        }(ipsToBlock)
    }
}

// processTick handles the sketch tick and IP blocking logic
func (cs *ConcurrentSketch) processTickWith(a *App, ip string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	cs.sketch.Incr(ip)
	cs.tickReq++
	
	if cs.tickReq >= cs.tickSize {
		cs.sketch.Tick()
		cs.tickCount++
		cs.tickReq = 0
		
		// Calculate threshold for this window
        // todo
		windowCapacity := uint64(cs.sketch.WindowSize) * cs.tickSize
		threshold := int((windowCapacity * thresholdPercent) / 100)
		
		// Get top items from sketch
		items := cs.sketch.SortedSlice()
		
		// Check items against threshold
		for _, item := range items {
			if item.Count > uint32(threshold) {
				if err := a.BlockIP(item.Item); err != nil {
				}
			} else {
				// Since list is sorted, we can break early
				break
			}
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
