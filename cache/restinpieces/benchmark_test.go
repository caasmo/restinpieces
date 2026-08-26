package restinpieces

import (
	"fmt"
	"testing"
	"time"
)

// Performance note: a TTL roughly doubles the cost per operation.
//
// The TTL write (SetWithTTL) and the expiry check in Get both call
// time.Now(), a vDSO clock read costing ~40ns. UnixNano() itself is
// negligible (~0.3ns); the clock read dominates. Measured on the dev
// machine (Intel i7-8550U, Linux, Go 1.26, 1000-entry cache):
//
//	GetHit       (no TTL)   ~39 ns/op   <- expiry check short-circuits
//	GetHitTTL    (with TTL) ~83 ns/op   <- +44 ns for time.Now()
//	SetNewKey                ~38 ns/op
//	SetWithTTL               ~98 ns/op   <- +60 ns for time.Now()
//
// So the clock read roughly doubles the per-op cost of both the read and
// write paths for TTL entries. A sampled clock (one package-level cached
// time value) is the planned fix; until then, time.Now() is used as-is.

const benchMaxEntries = 1000

// newBenchKeys returns n distinct string keys prefixed with prefix.
func newBenchKeys(prefix string, n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("%s-%d", prefix, i)
	}
	return keys
}

// fillBenchCache fills c with one entry per key.
func fillBenchCache(c *Cache[string, string], keys []string) {
	for _, k := range keys {
		c.Set(k, "value", 1)
	}
}

// BenchmarkCache_GetHit measures the full read path: map lookup plus the
// LRU move-to-head.
func BenchmarkCache_GetHit(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	fillBenchCache(c, keys)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(keys[i%benchMaxEntries])
	}
}

// BenchmarkCache_GetMiss measures the miss path: pure map lookup.
func BenchmarkCache_GetMiss(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	fillBenchCache(c, keys)
	missKeys := newBenchKeys("miss", benchMaxEntries)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(missKeys[i%benchMaxEntries])
	}
}

// BenchmarkCache_GetHitTTL measures the full read path on TTL entries:
// map lookup, LRU move-to-head, and the expiry check (time.Now()).
func BenchmarkCache_GetHitTTL(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	for _, k := range keys {
		c.SetWithTTL(k, "value", 1, time.Minute)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(keys[i%benchMaxEntries])
	}
}

// BenchmarkCache_SetWithFull measures the worst-case write path on a full
// cache: every Set inserts a new key and evicts the LRU tail.
func BenchmarkCache_SetWithFull(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	fillBenchCache(c, keys)
	churn := newBenchKeys("churn", benchMaxEntries)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(churn[i%benchMaxEntries], "value", 1)
	}
}

// BenchmarkCache_Overwrite measures the cheap write path: updating an
// existing key and moving it to the head.
func BenchmarkCache_Overwrite(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	fillBenchCache(c, keys)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(keys[i%benchMaxEntries], "value", 1)
	}
}

// BenchmarkCache_SetWithTTL measures the new-key write path with a TTL on
// a full cache.
func BenchmarkCache_SetWithTTL(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	fillBenchCache(c, keys)
	churn := newBenchKeys("churn", benchMaxEntries)

	const ttl = time.Minute
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.SetWithTTL(churn[i%benchMaxEntries], "value", 1, ttl)
	}
}

// BenchmarkCache_Mixed measures a read-heavy workload: 90% Get hits and
// 10% new-key Sets that evict.
func BenchmarkCache_Mixed(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	fillBenchCache(c, keys)
	churn := newBenchKeys("churn", benchMaxEntries)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%10 == 0 {
			c.Set(churn[(i/10)%benchMaxEntries], "value", 1)
		} else {
			c.Get(keys[i%benchMaxEntries])
		}
	}
}
