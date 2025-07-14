package topk

import (
	"sync"

	"github.com/keilerkonzept/topk/sliding"
)

// TopKSketch provides a thread-safe wrapper around a sliding window sketch
// for tracking frequent items and managing ticking.
const (
	thresholdPercent = 80 // 80% of window capacity
)

type TopKSketch struct {
	mu        sync.Mutex
	sketch    *sliding.Sketch
	tickSize  uint64 // number of request per tick
	tickReq   uint64 // Counter for requests processed since last tick
	tickCount uint64 // Counter for total ticks processed
	threshold int    // Precomputed threshold value
}

// New creates a new thread-safe sketch wrapper.
// It initializes the underlying sliding window sketch with the given parameters.
func New(window, segments, width, depth int, tickSize uint64) *TopKSketch {
	sketchInstance := sliding.New(window, segments, sliding.WithWidth(width), sliding.WithDepth(depth))

	if tickSize == 0 {
		tickSize = 1000 // Default tick size if not specified
	}

	windowCapacity := uint64(sketchInstance.WindowSize) * tickSize
	threshold := int((windowCapacity * thresholdPercent) / 100)

	return &TopKSketch{
		sketch:    sketchInstance,
		tickSize:  tickSize,
		threshold: threshold,
	}
}

// SizeBytes returns the memory usage of the sketch in bytes.
func (cs *TopKSketch) SizeBytes() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.sketch.SizeBytes()
}

func (cs *TopKSketch) ProcessTick(ip string) []string {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.sketch.Incr(ip)
	cs.tickReq++

	if cs.tickReq >= cs.tickSize {
		cs.sketch.Tick()
		cs.tickCount++
		cs.tickReq = 0

		items := cs.sketch.SortedSlice()

		ipsToBlock := make([]string, 0)
		for _, item := range items {
			if item.Count > uint32(cs.threshold) {
				ipsToBlock = append(ipsToBlock, item.Item)
			} else {
				break // Early exit due to sorted list
			}
		}
		return ipsToBlock // Return IPs to block
	}
	return nil // No blocking needed this tick
}
