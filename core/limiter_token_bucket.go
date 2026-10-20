package core

import (
	"github.com/caasmo/restinpieces/config"

	"golang.org/x/time/rate"
)

// PasswordHashing is the name of the built-in limiter that caps how many
// password hashes and verifications may run at the same time. Argon2id
// reserves 64 MiB per call, so without a cap a login flood can exhaust the
// server memory.
const PasswordHashing = "password_hashing"

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
