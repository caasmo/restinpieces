package core

import (
	"github.com/caasmo/restinpieces/config"

	"golang.org/x/time/rate"
)

// TokenBucketLimiter wraps a token bucket. Burst tokens are available right
// away and one token refills every Limit.
type TokenBucketLimiter struct {
	limiter *rate.Limiter
}

// NewTokenBucketLimiter returns a token bucket sized from entry.
func NewTokenBucketLimiter(entry config.TokenBucketEntry) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		limiter: rate.NewLimiter(rate.Every(entry.Limit.Duration), entry.Burst),
	}
}

// Limit returns true if the bucket has no token left, false otherwise. A call
// that returns false takes one token.
func (t *TokenBucketLimiter) Limit() bool {
	return !t.limiter.Allow()
}
