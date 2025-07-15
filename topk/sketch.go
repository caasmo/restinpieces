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
// it checks against the provided thresholds and returns a list of IPs to block.
func (cs *TopKSketch) ProcessTick(ip string, level string, activationRPS int) []string {
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
		// Determine maxSharePercent internally based on the level.
		var maxSharePercent int
		switch level {
		case "low":
			maxSharePercent = 50 // Lenient
		case "high":
			maxSharePercent = 20 // Aggressive
		default: // "medium"
			maxSharePercent = 35 // Balanced
		}

		windowCapacity := uint64(cs.sketch.WindowSize) * cs.tickSize
		thresholdCount := (windowCapacity * uint64(maxSharePercent)) / 100

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
