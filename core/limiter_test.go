package core

import (
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"golang.org/x/time/rate"
)

func newTestProvider(limiters config.Limiter) *config.Provider {
	cfg := config.NewDefaultConfig()
	cfg.Limiter = limiters
	return config.NewProvider(cfg)
}

func TestDefaultLimiters_Get(t *testing.T) {
	provider := newTestProvider(config.Limiter{
		TokenBucket: config.TokenBucket{
			PasswordHashing: {Limit: config.Duration{Duration: 2 * time.Second}, Burst: 5},
		},
	})
	limiters := NewDefaultLimiters(provider)

	first, err := limiters.Get(PasswordHashing)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	second, err := limiters.Get(PasswordHashing)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if first != second {
		t.Error("Get() returned a new limiter while the config entry did not change")
	}
}

func TestDefaultLimiters_GetNotDefined(t *testing.T) {
	limiters := NewDefaultLimiters(newTestProvider(config.Limiter{}))

	_, err := limiters.Get("upload")
	if err == nil {
		t.Fatal("Get() expected an error for an undefined name")
	}
}

func TestDefaultLimiter_Limit(t *testing.T) {
	limiter := &DefaultLimiter{limiter: rate.NewLimiter(rate.Every(time.Hour), 1)}

	if !limiter.Limit() {
		t.Error("Limit() = false, want true for the first call")
	}
	if limiter.Limit() {
		t.Error("Limit() = true, want false after the burst is exhausted")
	}
}
