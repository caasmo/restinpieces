package topk

import (
	"sort"
	"testing"
	"time"
)

// TestNew_Initialization tests that the New function correctly initializes the sketch.
// Its purpose is to ensure that the constructor properly sets the internal state
// of the TopKSketch based on the provided parameters.
func TestNew_Initialization(t *testing.T) {
	params := SketchParams{
		K:               10,
		WindowSize:      20,
		Width:           1024,
		Depth:           5,
		TickSize:        100,
		MaxSharePercent: 25,
		ActivationRPS:   500,
	}

	cs := New(params)

	if cs.tickSize != params.TickSize {
		t.Errorf("Expected tickSize to be %d, but got %d", params.TickSize, cs.tickSize)
	}
	if cs.maxSharePercent != params.MaxSharePercent {
		t.Errorf("Expected maxSharePercent to be %d, but got %d", params.MaxSharePercent, cs.maxSharePercent)
	}
	if cs.activationRPS != params.ActivationRPS {
		t.Errorf("Expected activationRPS to be %d, but got %d", params.ActivationRPS, cs.activationRPS)
	}
	if cs.sketch == nil {
		t.Errorf("Expected sketch to be initialized, but it was nil")
	}
}

// Request defines a single call to ProcessTick, allowing us to control timing.
type Request struct {
	ip    string        // The IP for this specific request
	sleep time.Duration // Delay after processing to simulate request rate
}

type testCase struct {
	name            string       // A descriptive name for the scenario
	params          SketchParams // Sketch configuration parameters
	requestSequence []Request    // Sequence of requests to simulate traffic
	wantBlockedIPs  []string     // Expected list of blocked IPs
}

// TestTopKSketch_ProcessTick is a table-driven test for the core logic of the sketch.
// Its purpose is to validate the behavior of the sketch under various traffic scenarios,
// ensuring it correctly implements the time-gated, high-share blocking logic.
func TestTopKSketch_ProcessTick(t *testing.T) {
	testCases := []testCase{
		{
			// Purpose: Verify that if not enough requests are made to complete a tick,
			// no blocking occurs. This is the simplest "do nothing" case.
			// With TickSize=100, but only 99 requests, the tick never completes,
			// so no blocking logic is triggered.
			name: "NoTick_ShouldNotBlock",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 100, MaxSharePercent: 20,
			},
			requestSequence: generateRequestSequence(0, map[string]int{"1.1.1.1": 99}),
			wantBlockedIPs: nil,
		},
		{
			// Purpose: This is a critical test for the circuit breaker's main gate.
			// It ensures that even if one IP is a top talker, it is NOT blocked
			// if the overall request rate is below the activation threshold.
			// With ActivationRPS=500, but the actions simulating only 400 RPS,
			// the blocker remains inactive.
			name: "LowRPS_TopkIP_ShouldNotBlock",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 500, MaxSharePercent: 20,
			},
			// Simulate 100 requests over 250ms (400 RPS), which is below the 500 RPS activation.
			requestSequence: generateRequestSequence(2*time.Millisecond, map[string]int{"1.1.1.1": 100}),
			wantBlockedIPs: nil,
		},
		{
			// Purpose: Verify that high server load alone does not trigger blocking
			// if the traffic is distributed and no single IP is consuming an unfair share.
			// This prevents false positives during legitimate traffic spikes.
			// The threshold is 20% of the total window capacity (10 * 100 = 1000), so 200 requests.
			// No IP exceeds this, so no blocking occurs.
			name: "HighRPS_NoTopkIP_ShouldNotBlock",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 500, MaxSharePercent: 20, // Threshold: 20% of 1000 = 200 requests
			},
			// Simulate 1000 RPS, but distribute them so none has > 20% share.
			requestSequence: generateRequestSequence(0, map[string]int{
				"1.1.1.1": 199, "2.2.2.2": 199, "3.3.3.3": 199,
				"4.4.4.4": 199, "5.5.5.5": 199, "6.6.6.6": 5,
			}),
			wantBlockedIPs: nil,
		},
		{
			// Purpose: Test the primary success case where the circuit breaker should trip.
			// The server is under high load, and a single IP is responsible for a
			// disproportionate amount of that load.
			// The threshold is 20% of the total window capacity (10 * 100 = 1000), so 200 requests.
			// IP 1.1.1.1 sends 201 requests, exceeding the threshold, and should be blocked.
			name: "HighRPS_SingleTopkIP_ShouldBlock",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 500, MaxSharePercent: 20, // Threshold: 20% of 1000 = 200 requests
			},
			// Simulate 1000 RPS, with one IP sending 201 requests.
			requestSequence: generateRequestSequence(0, map[string]int{"1.1.1.1": 201, "2.2.2.2": 199}),
			wantBlockedIPs: []string{"1.1.1.1"},
		},
		{
			// Purpose: Ensure the logic can identify and block multiple offenders in the
			// same window, not just the single top talker.
			// The threshold is 20% of the total window capacity (10 * 100 = 1000), so 200 requests.
			// Both 1.1.1.1 (201) and 2.2.2.2 (202) exceed this and should be blocked.
			name: "HighRPS_MultipleTopkIPs_ShouldBlockAll",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 500, MaxSharePercent: 20, // Threshold: 20% of 1000 = 200 requests
			},
			// Simulate 1000 RPS, with two IPs each sending > 200 requests.
			requestSequence: generateRequestSequence(0, map[string]int{
				"1.1.1.1": 201, "2.2.2.2": 202, "3.3.3.3": 597,
			}),
			wantBlockedIPs: []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"},
		},
		{
			// Purpose: This is an edge case test to ensure that if a tick happens
			// instantaneously (zero duration), the code doesn't panic due to division by zero.
			// The threshold is 10% of the window capacity (1000), so 100 requests.
			// IP 1.1.1.1 sends 101 requests and should be blocked.
			name: "InstantaneousTick_NoPanic",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 1, MaxSharePercent: 10, // Threshold: 10% of 1000 = 100 requests
			},
			// All actions have zero sleep, making the duration between ticks potentially zero.
			requestSequence: generateRequestSequence(0, map[string]int{"1.1.1.1": 101, "2.2.2.2": 899}),
			wantBlockedIPs: []string{"1.1.1.1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cs := New(tc.params)
			blockedIPs := make(map[string]struct{})

			for _, req := range tc.requestSequence {
				blocked := cs.ProcessTick(req.ip)
				if blocked != nil {
					for _, ip := range blocked {
						blockedIPs[ip] = struct{}{}
					}
				}
				if req.sleep > 0 {
					time.Sleep(req.sleep)
				}
			}

			gotBlockedIPs := make([]string, 0, len(blockedIPs))
			for ip := range blockedIPs {
				gotBlockedIPs = append(gotBlockedIPs, ip)
			}

			// Sort both slices for consistent comparison
			sort.Strings(gotBlockedIPs)
			sort.Strings(tc.wantBlockedIPs)

			if !equalSlices(gotBlockedIPs, tc.wantBlockedIPs) {
				t.Errorf("Test case '%s' failed: \n- got:  %v\n- want: %v", tc.name, gotBlockedIPs, tc.wantBlockedIPs)
			}
		})
	}
}

// equalSlices is a helper function to compare two string slices.
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// generateRequestSequence creates a sequence of requests for testing.
func generateRequestSequence(sleep time.Duration, counts map[string]int) []Request {
	var totalRequests int
	for _, count := range counts {
		totalRequests += count
	}
	requests := make([]Request, 0, totalRequests)
	for ip, count := range counts {
		for i := 0; i < count; i++ {
			requests = append(requests, Request{ip: ip, sleep: sleep})
		}
	}
	return requests
}

// interleaveRequestSequences mixes multiple request sequences to simulate real traffic patterns
func interleaveRequestSequences(seqs ...[]Request) []Request {
	var mixed []Request
	maxLen := 0
	for _, seq := range seqs {
		if len(seq) > maxLen {
			maxLen = len(seq)
		}
	}
	
	for i := 0; i < maxLen; i++ {
		for _, seq := range seqs {
			if i < len(seq) {
				mixed = append(mixed, seq[i])
			}
		}
	}
	return mixed
}
