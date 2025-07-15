package topk

import (
	"sync"
	"time"

	"github.com/keilerkonzept/topk/sliding"
)

// SketchParams holds the configuration for creating a new TopKSketch.
type SketchParams struct {
	K               int
	WindowSize      int
	Width           int
	Depth           int
	TickSize        uint64
	MaxSharePercent int
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
	}
}

// ProcessTick increments the count for the given item. If a tick completes,
// it checks against the provided thresholds and returns a list of IPs to block.
func (cs *TopKSketch) ProcessTick(ip string, activationRPS int) []string {
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
		if rps < float64(activationRPS) {
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
