package topk

import (
	"sync"
	"time"

	"github.com/keilerkonzept/topk/sliding"
)

// TopKSketch provides a thread-safe wrapper around a sliding window sketch
// for tracking frequent items and managing ticking.
type TopKSketch struct {
	mu           sync.Mutex
	sketch       *sliding.Sketch
	tickSize     uint64 // number of request per tick
	tickReq      uint64 // Counter for requests processed since last tick
	lastTickTime time.Time
}

// New creates a new thread-safe sketch wrapper.
// It initializes the underlying sliding window sketch with the given parameters.
func New(k, windowSize, width, depth int, tickSize uint64) *TopKSketch {
	sketchInstance := sliding.New(k, windowSize, sliding.WithWidth(width), sliding.WithDepth(depth))

	if tickSize == 0 {
		tickSize = 1000 // Default tick size if not specified
	}

	return &TopKSketch{
		sketch:       sketchInstance,
		tickSize:     tickSize,
		lastTickTime: time.Now(),
	}
}

// ProcessTick increments the count for the given item. If a tick completes,
// it returns the requests per second (RPS) for that tick and true.
// Otherwise, it returns 0 and false.
func (cs *TopKSketch) ProcessTick(ip string) (float64, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.sketch.Incr(ip)
	cs.tickReq++

	if cs.tickReq >= cs.tickSize {
		cs.sketch.Tick()
		cs.tickReq = 0

		now := time.Now()
		duration := now.Sub(cs.lastTickTime)
		cs.lastTickTime = now

		if duration.Seconds() <= 0 {
			return 0, true // Avoid division by zero, but signal that a tick occurred
		}

		rps := float64(cs.tickSize) / duration.Seconds()
		return rps, true
	}

	return 0, false
}

// TopItems returns a sorted slice of the most frequent items in the sketch.
func (cs *TopKSketch) TopItems() []sliding.Item {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.sketch.SortedSlice()
}

// WindowCapacity returns the total number of requests that a full window can hold.
func (cs *TopKSketch) WindowCapacity() uint64 {
	// This doesn't need a lock as sketch.WindowSize is constant after init.
	return uint64(cs.sketch.WindowSize) * cs.tickSize
}
