package topk

import (
	"reflect"
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

// testAction defines a single call to ProcessTick, allowing us to control timing.
type testAction struct {
	ip    string        // The IP for this specific request.
	sleep time.Duration // How long to wait *after* this request to simulate traffic rate.
}

// processTickTestCase defines a complete scenario for the table-driven test.
type processTickTestCase struct {
	name           string       // A descriptive name for the scenario.
	params         SketchParams // The configuration to initialize the sketch with.
	actions        []testAction // A sequence of calls to ProcessTick to simulate traffic.
	wantBlockedIPs []string     // The expected list of IPs to be blocked at the end of the sequence.
}

// TestTopKSketch_ProcessTick is a table-driven test for the core logic of the sketch.
// Its purpose is to validate the behavior of the sketch under various traffic scenarios,
// ensuring it correctly implements the time-gated, high-share blocking logic.
func TestTopKSketch_ProcessTick(t *testing.T) {
	testCases := []processTickTestCase{
		{
			// Purpose: Verify that if not enough requests are made to complete a tick,
			// no blocking occurs. This is the simplest "do nothing" case.
			name: "NoTick_ShouldNotBlock",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 100, MaxSharePercent: 20,
			},
			actions:        generateActions(99, 0, map[string]int{"1.1.1.1": 99}),
			wantBlockedIPs: nil,
		},
		{
			// Purpose: This is a critical test for the circuit breaker's main gate.
			// It ensures that even if one IP is completely dominant, it is NOT blocked
			// if the overall request rate is below the activation threshold.
			name: "LowRPS_DominantIP_ShouldNotBlock",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 500, MaxSharePercent: 20,
			},
			// Simulate 100 requests over 250ms (400 RPS), which is below the 500 RPS activation.
			actions:        generateActions(100, 2*time.Millisecond, map[string]int{"1.1.1.1": 100}),
			wantBlockedIPs: nil,
		},
		{
			// Purpose: Verify that high server load alone does not trigger blocking
			// if the traffic is distributed and no single IP is consuming an unfair share.
			// This prevents false positives during legitimate traffic spikes.
			name: "HighRPS_NoDominantIP_ShouldNotBlock",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 500, MaxSharePercent: 20, // Threshold: 20% of 1000 = 200 requests
			},
			// Simulate 1000 RPS, but distribute them so none has > 20% share.
			actions: generateActions(1000, 0, map[string]int{
				"1.1.1.1": 199, "2.2.2.2": 199, "3.3.3.3": 199,
				"4.4.4.4": 199, "5.5.5.5": 199, "6.6.6.6": 5,
			}),
			wantBlockedIPs: nil,
		},
		{
			// Purpose: Test the primary success case where the circuit breaker should trip.
			// The server is under high load, and a single IP is responsible for a
			// disproportionate amount of that load.
			name: "HighRPS_SingleDominantIP_ShouldBlock",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 500, MaxSharePercent: 20, // Threshold: 20% of 1000 = 200 requests
			},
			// Simulate 1000 RPS, with one IP sending 201 requests.
			actions:        generateActions(1000, 0, map[string]int{"1.1.1.1": 201, "2.2.2.2": 799}),
			wantBlockedIPs: []string{"1.1.1.1"},
		},
		{
			// Purpose: Ensure the logic can identify and block multiple offenders in the
			// same window, not just the single top talker.
			name: "HighRPS_MultipleDominantIPs_ShouldBlockAll",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 500, MaxSharePercent: 20, // Threshold: 20% of 1000 = 200 requests
			},
			// Simulate 1000 RPS, with two IPs each sending > 200 requests.
			actions: generateActions(1000, 0, map[string]int{
				"1.1.1.1": 201, "2.2.2.2": 202, "3.3.3.3": 597,
			}),
			wantBlockedIPs: []string{"1.1.1.1", "2.2.2.2"},
		},
		{
			// Purpose: Verify that the sketch's internal state (lastTickTime, window)
			// is correctly managed across multiple, distinct ticks.
			name: "StateAcrossMultipleTicks",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 500, MaxSharePercent: 20, // Threshold: 20% of 1000 = 200 requests
			},
			actions: combineActions(
				// Tick 1: High RPS, IP 1.1.1.1 is dominant and should be blocked.
				generateActions(1000, 0, map[string]int{"1.1.1.1": 300, "2.2.2.2": 700}),
				// Tick 2: Low RPS, IP 3.3.3.3 is dominant but should NOT be blocked.
				generateActions(100, 3*time.Millisecond, map[string]int{"3.3.3.3": 90, "4.4.4.4": 10}),
				// Tick 3: High RPS again, IP 5.5.5.5 is now dominant and should be blocked.
				generateActions(1000, 0, map[string]int{"5.5.5.5": 400, "6.6.6.6": 600}),
			),
			// We only expect the IPs from the high-RPS ticks to be blocked.
			wantBlockedIPs: []string{"1.1.1.1", "5.5.5.5"},
		},
		{
			// Purpose: This is an edge case test to ensure that if a tick happens
			// instantaneously (zero duration), the code doesn't panic due to division by zero.
			name: "InstantaneousTick_NoPanic",
			params: SketchParams{
				K: 5, WindowSize: 10, Width: 1024, Depth: 3, TickSize: 100,
				ActivationRPS: 1, MaxSharePercent: 10, // Threshold: 10% of 1000 = 100 requests
			},
			// All actions have zero sleep, making the duration between ticks potentially zero.
			actions:        generateActions(1000, 0, map[string]int{"1.1.1.1": 101, "2.2.2.2": 899}),
			wantBlockedIPs: []string{"1.1.1.1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cs := New(tc.params)
			var allBlockedIPs []string

			for _, action := range tc.actions {
				blocked := cs.ProcessTick(action.ip)
				if blocked != nil {
					allBlockedIPs = append(allBlockedIPs, blocked...)
				}
				if action.sleep > 0 {
					time.Sleep(action.sleep)
				}
			}

			// Sort both slices for consistent comparison
			sort.Strings(allBlockedIPs)
			sort.Strings(tc.wantBlockedIPs)

			if !reflect.DeepEqual(allBlockedIPs, tc.wantBlockedIPs) {
				t.Errorf("Test case '%s' failed: \n- got:  %v\n- want: %v", tc.name, allBlockedIPs, tc.wantBlockedIPs)
			}
		})
	}
}

// generateActions is a helper function to create a sequence of test actions.
func generateActions(totalActions int, sleep time.Duration, counts map[string]int) []testAction {
	actions := make([]testAction, 0, totalActions)
	for ip, count := range counts {
		for i := 0; i < count; i++ {
			actions = append(actions, testAction{ip: ip, sleep: sleep})
		}
	}
	// Ensure the total number of actions is met, filling with a generic IP if needed.
	for len(actions) < totalActions {
		actions = append(actions, testAction{ip: "9.9.9.9", sleep: sleep})
	}
	return actions
}

// combineActions is a helper to merge multiple action sequences for multi-tick tests.
func combineActions(actionLists ...[]testAction) []testAction {
	var combined []testAction
	for _, list := range actionLists {
		combined = append(combined, list...)
	}
	return combined
}
