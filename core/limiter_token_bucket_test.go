package core

import (
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
)

func TestTokenBucketLimiter_Limit(t *testing.T) {
	limiter := NewTokenBucketLimiter(config.TokenBucketEntry{
		Limit: config.Duration{Duration: time.Hour},
		Burst: 1,
	})

	if limiter.Limit() {
		t.Error("Limit() = true, want false for the first call")
	}
	if !limiter.Limit() {
		t.Error("Limit() = false, want true after the burst is exhausted")
	}
}

func TestTokenBucketLimiter_Burst(t *testing.T) {
	limiter := NewTokenBucketLimiter(config.TokenBucketEntry{
		Limit: config.Duration{Duration: time.Hour},
		Burst: 2,
	})

	if limiter.Limit() {
		t.Error("first Limit() = true, want false")
	}
	if limiter.Limit() {
		t.Error("second Limit() = true, want false")
	}
	if !limiter.Limit() {
		t.Error("third Limit() = false, want true")
	}
}
