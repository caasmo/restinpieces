package core

import (
	"testing"
	"time"

	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/config"
)

func newTestCounterCache(tb testing.TB) *cache.Default[string, any] {
	tb.Helper()

	counterCache, err := cache.New[any]("small")
	if err != nil {
		tb.Fatalf("cache.New() error = %v", err)
	}

	return counterCache
}

func TestFixedWindowLimiter_Limit(t *testing.T) {
	entries := config.FixedWindow{
		"password_reset": {
			Prefix: "pr_",
			Window: config.Duration{Duration: time.Hour},
			Max:    2,
		},
	}
	limiter := NewFixedWindowLimiter(entries, newTestCounterCache(t))

	if limiter.Limit("password_reset", "user@example.com") {
		t.Error("first Limit() = true, want false")
	}
	if limiter.Limit("password_reset", "user@example.com") {
		t.Error("second Limit() = true, want false")
	}
	if !limiter.Limit("password_reset", "user@example.com") {
		t.Error("third Limit() = false, want true after max is reached")
	}
}

func TestFixedWindowLimiter_PerIdentity(t *testing.T) {
	entries := config.FixedWindow{
		"password_reset": {
			Prefix: "pr_",
			Window: config.Duration{Duration: time.Hour},
			Max:    1,
		},
	}
	limiter := NewFixedWindowLimiter(entries, newTestCounterCache(t))

	if limiter.Limit("password_reset", "first@example.com") {
		t.Error("Limit() = true for the first identity, want false")
	}
	if limiter.Limit("password_reset", "second@example.com") {
		t.Error("Limit() = true for the second identity, want false")
	}
	if !limiter.Limit("password_reset", "first@example.com") {
		t.Error("Limit() = false for a spent identity, want true")
	}
}

func TestFixedWindowLimiter_UndefinedLabel(t *testing.T) {
	limiter := NewFixedWindowLimiter(config.FixedWindow{}, newTestCounterCache(t))

	if !limiter.Limit("upload", "user@example.com") {
		t.Error("Limit() = false for an undefined label, want true")
	}
}

func TestFixedWindowKey(t *testing.T) {
	got := fixedWindowKey("pr_", "user@example.com", 42)
	want := "pr_user@example.com_42"

	if got != want {
		t.Errorf("fixedWindowKey() = %q, want %q", got, want)
	}
}
