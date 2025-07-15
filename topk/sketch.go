package topk

import (
	"sync"
	"time"

	"github.com/keilerkonzept/topk/sliding"
)

// SketchParams holds the configuration for creating a new TopKSketch.
type SketchParams struct {
	// K is the number of top items to keep track of in the sketch.
	K int
	// WindowSize is the size of the sliding window, measured in ticks. The total
	// theoretical capacity of the window is `WindowSize * TickSize`. For example,
	// if WindowSize is 10 and TickSize is 100, the window capacity is 1000 requests.
	WindowSize int
	// Width is the width of the underlying Count-Min sketch. A larger width
	// reduces the probability of over-counting but increases memory usage.
	Width int
	// Depth is the depth of the underlying Count-Min sketch. A larger depth
	// also reduces over-counting at the cost of more memory.
	Depth int
	// TickSize is the number of requests that constitute a single "tick". After
	// this many requests, the sketch's internal clock advances.
	TickSize uint64
	// MaxSharePercent is the maximum percentage of the total window capacity that
	// a single IP can consume before being considered for blocking. This logic prevents
	// server breakdown by allowing a higher share for lower traffic levels (where a
	// dominant IP is not a threat) and a lower, more aggressive share for higher
	// traffic levels. For example, at the 'medium' level (35% share, 1000 request
	// capacity), an IP is blocked if it exceeds 350 requests within the window.
	MaxSharePercent int
	// ActivationRPS is the requests-per-second threshold that must be met for the
	// blocker to become active. Its primary purpose is to act as a gate, ensuring
	// the blocker does nothing during periods of low server load. For example, at the
	// 'medium' level (100 request TickSize, 500 RPS activation), a tick must occur
	// in 200ms or less for the blocker to engage.
	ActivationRPS int
}

// TopKSketch provides a thread-safe wrapper around a sliding window sketch
// for tracking frequent items and managing ticking.
type TopKSketch struct {
	mu              sync.Mutex
	sketch          *sliding.Sketch
	tickSize        uint64 // number of request per tick
	tickReq         uint64 // Counter for requests processed since last tick
	lastTickTime    time.Time
	maxSharePercent int
	activationRPS   int
}

// New creates a new thread-safe sketch wrapper.
// It initializes the underlying sliding window sketch with the given parameters.
func New(params SketchParams) *TopKSketch {
	sketchInstance := sliding.New(params.K, params.WindowSize, sliding.WithWidth(params.Width), sliding.WithDepth(params.Depth))

	return &TopKSketch{
		sketch:          sketchInstance,
		tickSize:        params.TickSize,
		lastTickTime:    time.Now(),
		maxSharePercent: params.MaxSharePercent,
		activationRPS:   params.ActivationRPS,
	}
}

// ProcessTick increments the count for the given item. If a tick completes,
// it checks against the provided thresholds and returns a list of IPs to block.
func (cs *TopKSketch) ProcessTick(ip string) []string {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.sketch.Incr(ip)
	cs.tickReq++

	if cs.tickReq >= cs.tickSize {
		// A tick has completed, now we check the conditions for blocking.
		cs.tickReq = 0
		now := time.Now()
		duration := now.Sub(cs.lastTickTime)
		cs.lastTickTime = now

		var rps float64
		if duration.Seconds() > 0 {
			rps = float64(cs.tickSize) / duration.Seconds()
		}

		// --- Gate 1: Is the server busy enough? ---
		if rps < float64(cs.activationRPS) {
			cs.sketch.Tick() // Still tick the sketch to slide the window, but don't block.
			return nil
		}

		// --- Gate 2: Is any IP consuming too much? ---
		windowCapacity := uint64(cs.sketch.WindowSize) * cs.tickSize
		thresholdCount := (windowCapacity * uint64(cs.maxSharePercent)) / 100

		itemsToBlock := make([]string, 0)
		// We check the items *before* ticking to evaluate the window that just completed.
		for _, item := range cs.sketch.SortedSlice() {
			if item.Count > uint32(thresholdCount) {
				itemsToBlock = append(itemsToBlock, item.Item)
			} else {
				break // Sorted list allows early exit.
			}
		}

		cs.sketch.Tick() // Now, slide the window.
		return itemsToBlock
	}

	return nil
}
