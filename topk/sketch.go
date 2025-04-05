package topk

import (
	"sync"

	"github.com/keilerkonzept/topk/sliding"
)

// ConcurrentSketch provides thread-safe access to a sketch instance and manages ticking.
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

// NewConcurrentSketch creates a new thread-safe sketch wrapper.
// tickSize: How many requests trigger a sketch tick and top-k check.
// TODO reove reference to Ips
func NewTopkSketch(instance *sliding.Sketch, tickSize uint64) *TopKSketch {
	if instance == nil {
		panic("sketch instance cannot be nil for ConcurrentSketch")
	}
	if tickSize == 0 {
		tickSize = 1000 // Default tick size if not specified
	}

	windowCapacity := uint64(instance.WindowSize) * tickSize
	threshold := int((windowCapacity * thresholdPercent) / 100)

	return &TopKSketch{
		sketch:    instance,
		tickSize:  tickSize,
		threshold: threshold,
	}
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
