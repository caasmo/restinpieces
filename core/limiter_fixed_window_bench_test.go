package core

import (
	"math"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
)

// benchMaxIdentities is the number of identities the benchmarks cycle
// through, so every call reaches an existing key instead of growing the cache.
const benchMaxIdentities = 1000

func benchIdentities() []string {
	identities := make([]string, benchMaxIdentities)
	for i := range identities {
		identities[i] = "user" + strconv.Itoa(i) + "@example.com"
	}

	return identities
}

// BenchmarkFixedWindowLimiter_Limit_Serial measures a call that passes the
// limiter. Every call works on a counter that already exists, and the limit is
// set high so the call passes for the whole run. The result is the time one
// normal request spends in the limiter.
func BenchmarkFixedWindowLimiter_Limit_Serial(b *testing.B) {
	entries := config.FixedWindow{
		"password_reset": {
			Prefix: "pr_",
			Window: config.Duration{Duration: time.Hour},
			Max:    math.MaxInt64,
		},
	}
	limiter := NewFixedWindowLimiter(entries, newTestCounterCache(b))
	identities := benchIdentities()

	i := 0
	for b.Loop() {
		if limiter.Limit("password_reset", identities[i%len(identities)]) {
			b.Fatal("Limit() = true, want false")
		}
		i++
	}
}

// BenchmarkFixedWindowLimiter_LimitRefused_Serial measures a call the limiter
// refuses. Every identity has already used up its window, so each call still
// updates its counter and is refused. The result is the time one repeated
// request spends in the limiter once the limit is reached.
func BenchmarkFixedWindowLimiter_LimitRefused_Serial(b *testing.B) {
	entries := config.FixedWindow{
		"password_reset": {
			Prefix: "pr_",
			Window: config.Duration{Duration: time.Hour},
			Max:    1,
		},
	}
	limiter := NewFixedWindowLimiter(entries, newTestCounterCache(b))
	identities := benchIdentities()

	for _, identity := range identities {
		if limiter.Limit("password_reset", identity) {
			b.Fatalf("Limit(%q) = true before the benchmark, want false", identity)
		}
	}

	i := 0
	for b.Loop() {
		if !limiter.Limit("password_reset", identities[i%len(identities)]) {
			b.Fatal("Limit() = false, want true after max is reached")
		}
		i++
	}
}

// BenchmarkFixedWindowLimiter_Limit_Parallel measures calls that pass the
// limiter while several goroutines share one limiter. The limit is set high so
// the call passes for the whole run. The result is how many calls run per second
// and what the shared cache costs when it must lock.
func BenchmarkFixedWindowLimiter_Limit_Parallel(b *testing.B) {
	entries := config.FixedWindow{
		"password_reset": {
			Prefix: "pr_",
			Window: config.Duration{Duration: time.Hour},
			Max:    math.MaxInt64,
		},
	}
	limiter := NewFixedWindowLimiter(entries, newTestCounterCache(b))
	identities := benchIdentities()

	var next atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			identity := identities[int(next.Add(1))%len(identities)]
			if limiter.Limit("password_reset", identity) {
				b.Error("Limit() = true, want false")
			}
		}
	})
}

// BenchmarkFixedWindowKey_Build measures only the key string, without the
// cache. The result shows that building a key allocates memory once per call,
// so a change that adds an allocation is visible.
func BenchmarkFixedWindowKey_Build(b *testing.B) {
	for b.Loop() {
		_ = fixedWindowKey("pr_", "user@example.com", 493827)
	}
}
