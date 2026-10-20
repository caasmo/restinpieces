package core

// Limiter answers rate-limit checks. Both branches are built by the caller at
// startup; Limit is the only method callers use. Changing the limiter config
// needs a restart.
type Limiter struct {
	TokenBucket map[string]*TokenBucketLimiter
	FixedWindow *FixedWindowLimiter
}

// NewLimiter assembles a Limiter from already-built branches. The caller
// creates every token bucket and the shared fixed-window object, so Limit
// never writes the maps.
func NewLimiter(
	tokenBuckets map[string]*TokenBucketLimiter,
	fixedWindow *FixedWindowLimiter,
) *Limiter {
	return &Limiter{
		TokenBucket: tokenBuckets,
		FixedWindow: fixedWindow,
	}
}

// Limit returns true if the call named by label is refused, false otherwise.
// A token-bucket label ignores identities; every other label needs exactly one
// identity. A label configured in both maps is served by its token bucket.
func (l *Limiter) Limit(label string, identities ...string) bool {
	if tokenBucket, exists := l.TokenBucket[label]; exists {
		return tokenBucket.Limit()
	}

	if len(identities) != 1 {
		return true
	}

	return l.FixedWindow.Limit(label, identities[0])
}
