package cache

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"
)

// Performance note: TTL cost with sampled clock.
//
// The TTL write (SetWithTTL) and the expiry check in Get use fastNow —
// a periodically-refreshed snapshot of time.Now().UnixNano() (clockNow)
// via an atomic load (~1-2ns) instead of a direct vDSO clock read
// (~40ns). Non-TTL entries short-circuit on expiration == 0 and never
// touch the clock. Measured on the dev machine (Intel i7-8550U, Linux,
// Go 1.27, 1000-entry cache, fastNow):
//
//	GetHit       (no TTL)   ~38 ns/op   <- expiry check short-circuits
//	GetHitTTL    (with TTL) ~40 ns/op   <- +2 ns for fastNow
//	SetOverwrite             ~40 ns/op
//	SetWithFull             ~125 ns/op   <- eviction + map delete
//	SetWithTTLFull          ~140 ns/op   <- +fastNow

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
func fillBenchCache(c *Default[string, string], keys []string) {
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
	i := 0
	for b.Loop() {
		c.Get(keys[i%benchMaxEntries])
		i++
	}
}

// BenchmarkCache_GetHitHead measures the cheap hit path on a realistic cache:
// the cache holds benchMaxEntries keys and the hot key is already the LRU
// head, so Get returns without the unlink/linkToHead move. The map lookup
// matches BenchmarkCache_GetHit.
func BenchmarkCache_GetHitHead(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries-1)
	fillBenchCache(c, keys)
	c.Set("head", "value", 1)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.Get("head")
	}
}

// BenchmarkCache_GetHitAny measures the full read path with V = any, the
// value type the framework ships.
func BenchmarkCache_GetHitAny(b *testing.B) {
	c := newWith[string, any](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	for _, key := range keys {
		c.Set(key, "value", 1)
	}

	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		c.Get(keys[i%benchMaxEntries])
		i++
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
	i := 0
	for b.Loop() {
		c.Get(missKeys[i%benchMaxEntries])
		i++
	}
}

// BenchmarkCache_GetHitTTL measures the full read path on TTL entries:
// map lookup, LRU move-to-head, and the expiry check (fastNow). The TTL is
// an hour so entries stay live for any benchmark run.
func BenchmarkCache_GetHitTTL(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	for _, k := range keys {
		c.SetWithTTL(k, "value", 1, time.Hour)
	}

	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		c.Get(keys[i%benchMaxEntries])
		i++
	}
}

// benchSet measures the plain insert path with free nodes available for
// value type V. The cache is emptied in batches so the timed path never
// fills.
func benchSet[V any](b *testing.B, value V) {
	c := newWith[string, V](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)

	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		if i == len(keys) {
			b.StopTimer()
			clear(c.index)
			c.head = -1
			c.tail = -1
			c.free = c.free[:0]
			for n := int32(0); n < int32(benchMaxEntries); n++ {
				c.free = append(c.free, n)
			}
			b.StartTimer()
			i = 0
		}
		c.Set(keys[i], value, 1)
		i++
	}
}

// BenchmarkCache_Set measures the plain insert path with free nodes
// available: one lock, a map miss, a free node, and insertion, with no
// eviction.
func BenchmarkCache_Set(b *testing.B) {
	benchSet(b, "value")
}

// BenchmarkCache_SetAny measures the same insert path with V = any, the
// value type the framework ships.
func BenchmarkCache_SetAny(b *testing.B) {
	benchSet[any](b, "value")
}

// BenchmarkCache_SetWithFull measures the worst-case write path on a full
// cache: every Set inserts a new key and evicts the LRU tail.
func BenchmarkCache_SetWithFull(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	fillBenchCache(c, keys)
	// The churn set is larger than the cache, so every call misses and
	// evicts a node.
	churn := newBenchKeys("churn", 2*benchMaxEntries)

	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		c.Set(churn[i%len(churn)], "value", 1)
		i++
	}
}

// BenchmarkCache_SetWithFullAny measures the full-cache insert path with
// V = any, the value type the framework ships.
func BenchmarkCache_SetWithFullAny(b *testing.B) {
	c := newWith[string, any](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	for _, key := range keys {
		c.Set(key, "value", 1)
	}
	// The churn set is larger than the cache, so every call misses and
	// evicts a node.
	churn := newBenchKeys("churn", 2*benchMaxEntries)

	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		c.Set(churn[i%len(churn)], "value", 1)
		i++
	}
}

// BenchmarkCache_SetOverwrite measures the cheap write path: updating an
// existing key and moving it to the head.
func BenchmarkCache_SetOverwrite(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	fillBenchCache(c, keys)

	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		c.Set(keys[i%benchMaxEntries], "value", 1)
		i++
	}
}

// BenchmarkCache_SetOverwriteHead measures the cheap overwrite path on a
// realistic cache: the cache holds benchMaxEntries keys and the hot key is
// already the LRU head, so Set skips the unlink/linkToHead move. The map
// lookup matches BenchmarkCache_SetOverwrite.
func BenchmarkCache_SetOverwriteHead(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries-1)
	fillBenchCache(c, keys)
	c.Set("head", "value", 1)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.Set("head", "value", 1)
	}
}

// BenchmarkCache_SetOverwriteAny measures the overwrite path with V = any,
// the value type the framework ships.
func BenchmarkCache_SetOverwriteAny(b *testing.B) {
	c := newWith[string, any](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	for _, key := range keys {
		c.Set(key, "value", 1)
	}

	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		c.Set(keys[i%benchMaxEntries], "value", 1)
		i++
	}
}

// BenchmarkCache_IncrementAny measures the found path with V = any: one lock,
// a map hit, the uint64 addition, and the LRU move. Storing a count of 256
// or more boxes the uint64 and allocates 8 bytes.
func BenchmarkCache_IncrementAny(b *testing.B) {
	c := newWith[string, any](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	for _, key := range keys {
		_, _ = c.Increment(key, 1)
	}

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_, _ = c.Increment(keys[i%benchMaxEntries], 1)
		i++
	}
}

// BenchmarkCache_IncrementAnyHead measures the found path with V = any on a
// realistic cache: the cache holds benchMaxEntries counters and the hot
// counter is already the LRU head, so Increment skips the move. The map
// lookup matches BenchmarkCache_IncrementAny.
func BenchmarkCache_IncrementAnyHead(b *testing.B) {
	c := newWith[string, any](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries-1)
	for _, key := range keys {
		_, _ = c.Increment(key, 1)
	}
	_, _ = c.Increment("head", 1)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = c.Increment("head", 1)
	}
}

// BenchmarkCache_IncrementUint64 measures the same found path with V =
// uint64. Compare its allocs/op with BenchmarkCache_IncrementAny to see
// whether the compiler elides the any(count).(V) round-trip.
func BenchmarkCache_IncrementUint64(b *testing.B) {
	c := newWith[string, uint64](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	for _, key := range keys {
		_, _ = c.Increment(key, 1)
	}

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_, _ = c.Increment(keys[i%benchMaxEntries], 1)
		i++
	}
}

// BenchmarkCache_IncrementFull measures the create path on a full cache:
// one lock, a map miss, tail eviction, and insertion.
func BenchmarkCache_IncrementFull(b *testing.B) {
	c := newWith[string, any](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	for _, key := range keys {
		_, _ = c.Increment(key, 1)
	}
	// The churn set is larger than the cache, so every call misses and
	// evicts a node.
	churn := newBenchKeys("churn", 2*benchMaxEntries)

	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_, _ = c.Increment(churn[i%len(churn)], 1)
		i++
	}
}

// BenchmarkCache_SetWithTTLFull measures the new-key write path with a TTL
// on a full cache.
func BenchmarkCache_SetWithTTLFull(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", benchMaxEntries)
	fillBenchCache(c, keys)
	// The churn set is larger than the cache, so every call misses and
	// evicts a node.
	churn := newBenchKeys("churn", 2*benchMaxEntries)

	const ttl = time.Minute
	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		c.SetWithTTL(churn[i%len(churn)], "value", 1, ttl)
		i++
	}
}

// BenchmarkCache_Mixed90Get10Set measures a read-heavy workload: 90% Gets
// and 10% Sets over a randomized key universe twice the cache size. The
// reported %hit is the measured Get hit rate for the run.
func BenchmarkCache_Mixed90Get10Set(b *testing.B) {
	c := newWith[string, string](benchMaxEntries)
	keys := newBenchKeys("key", 2*benchMaxEntries)
	fillBenchCache(c, keys[:benchMaxEntries])
	rng := rand.New(rand.NewPCG(1, 2))

	b.ReportAllocs()
	b.ResetTimer()
	hits, gets := 0, 0
	for b.Loop() {
		v := rng.Uint64()
		key := keys[(v/10)%uint64(len(keys))]
		if v%10 == 0 {
			c.Set(key, "value", 1)
			continue
		}
		gets++
		if _, ok := c.Get(key); ok {
			hits++
		}
	}
	if gets > 0 {
		b.ReportMetric(100*float64(hits)/float64(gets), "%hit")
	}
}
