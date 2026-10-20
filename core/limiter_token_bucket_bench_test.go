package core

import (
	"math"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
)

// BenchmarkTokenBucketLimiter_Limit_Serial measures a call that passes the
// limiter. A zero refill interval means the bucket refills with no limit, so the
// call passes for the whole run. The result is the time one normal request
// spends in the limiter.
func BenchmarkTokenBucketLimiter_Limit_Serial(b *testing.B) {
	limiter := NewTokenBucketLimiter(config.TokenBucketEntry{
		Limit: config.Duration{},
		Burst: math.MaxInt,
	})

	for b.Loop() {
		if limiter.Limit() {
			b.Fatal("Limit() = true, want false")
		}
	}
}

// BenchmarkTokenBucketLimiter_LimitRefused_Serial measures a call the limiter
// refuses. A burst of zero means the bucket never holds a token. The result is
// the time one repeated request spends in the limiter once the limit is
// reached.
func BenchmarkTokenBucketLimiter_LimitRefused_Serial(b *testing.B) {
	limiter := NewTokenBucketLimiter(config.TokenBucketEntry{
		Limit: config.Duration{Duration: time.Hour},
		Burst: 0,
	})

	for b.Loop() {
		if !limiter.Limit() {
			b.Fatal("Limit() = false, want true")
		}
	}
}

// BenchmarkTokenBucketLimiter_Limit_Parallel measures calls that pass the
// limiter while several goroutines share one limiter. A zero refill interval
// means the bucket refills with no limit, so every call passes. The result is how
// many calls run per second across goroutines.
func BenchmarkTokenBucketLimiter_Limit_Parallel(b *testing.B) {
	limiter := NewTokenBucketLimiter(config.TokenBucketEntry{
		Limit: config.Duration{},
		Burst: math.MaxInt,
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if limiter.Limit() {
				b.Error("Limit() = true, want false")
			}
		}
	})
}
