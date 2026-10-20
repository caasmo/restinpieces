package core

import (
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
)

func TestNewLimiter(t *testing.T) {
	tokenBuckets := map[string]*TokenBucketLimiter{
		"upload": NewTokenBucketLimiter(config.TokenBucketEntry{
			Limit: config.Duration{Duration: time.Hour},
			Burst: 2,
		}),
	}
	fixedWindow := NewFixedWindowLimiter(config.FixedWindow{
		"password_reset": {
			Prefix: "pr_",
			Window: config.Duration{Duration: time.Hour},
			Max:    1,
		},
	}, newTestCounterCache(t))

	limiter := NewLimiter(tokenBuckets, fixedWindow)

	if _, ok := limiter.TokenBucket["upload"]; !ok {
		t.Error("TokenBucket does not contain upload")
	}
	if limiter.FixedWindow != fixedWindow {
		t.Error("FixedWindow does not hold the fixed-window object passed to NewLimiter")
	}
}

func TestLimiter_LimitFixedWindow(t *testing.T) {
	entries := config.FixedWindow{
		"password_reset": {
			Prefix: "pr_",
			Window: config.Duration{Duration: time.Hour},
			Max:    1,
		},
	}
	fixedWindow := NewFixedWindowLimiter(entries, newTestCounterCache(t))
	limiter := NewLimiter(map[string]*TokenBucketLimiter{}, fixedWindow)

	if limiter.Limit("password_reset", "user@example.com") {
		t.Error("first Limit() = true, want false")
	}
	if !limiter.Limit("password_reset", "user@example.com") {
		t.Error("second Limit() = false, want true after max is reached")
	}
}

func TestLimiter_LimitFixedWindowIdentityCount(t *testing.T) {
	limiter := NewLimiter(
		map[string]*TokenBucketLimiter{},
		NewFixedWindowLimiter(config.FixedWindow{}, nil),
	)

	testCases := []struct {
		name       string
		identities []string
	}{
		{name: "no identity"},
		{name: "two identities", identities: []string{"first@example.com", "second@example.com"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if !limiter.Limit("password_reset", tc.identities...) {
				t.Error("Limit() = false with the wrong identity count, want true")
			}
		})
	}
}

func TestLimiter_LimitTokenBucket(t *testing.T) {
	tokenBuckets := map[string]*TokenBucketLimiter{
		"upload": NewTokenBucketLimiter(config.TokenBucketEntry{
			Limit: config.Duration{Duration: time.Hour},
			Burst: 2,
		}),
	}
	limiter := NewLimiter(tokenBuckets, NewFixedWindowLimiter(config.FixedWindow{}, nil))

	if limiter.Limit("upload") {
		t.Error("first Limit() = true, want false")
	}
	if limiter.Limit("upload", "ignored@example.com") {
		t.Error("second Limit() with an identity = true, want false")
	}
	if !limiter.Limit("upload") {
		t.Error("third Limit() = false, want true after the burst is gone")
	}
}

func TestLimiter_LimitUndefinedLabel(t *testing.T) {
	limiter := NewLimiter(
		map[string]*TokenBucketLimiter{},
		NewFixedWindowLimiter(config.FixedWindow{}, nil),
	)

	if !limiter.Limit("upload") {
		t.Error("Limit() = false for an undefined label, want true")
	}
}
