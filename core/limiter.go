package core

import (
	"fmt"
	"sync"

	"github.com/caasmo/restinpieces/config"

	"golang.org/x/time/rate"
)

// PasswordHashing is the name of the built-in limiter that caps how many
// password hashes and verifications may run at the same time. Argon2id
// reserves 64 MiB per call, so without a cap a login flood can exhaust the
// server memory.
const PasswordHashing = "password_hashing"

// Limiter allows or refuses one call.
type Limiter interface {
	Limit() bool
}

// Limiters returns the limiter registered under name. A name the config does
// not define is an error.
type Limiters interface {
	Get(name string) (Limiter, error)
}

// DefaultLimiter wraps a token bucket. Burst tokens are available right away
// and one token refills every Limit.
type DefaultLimiter struct {
	limiter *rate.Limiter
}

// Limit reports whether a token is available.
func (d *DefaultLimiter) Limit() bool {
	return d.limiter.Allow()
}

// defaultLimiters builds one DefaultLimiter per configured name on first use.
type defaultLimiters struct {
	config   *config.Provider
	mu       sync.Mutex
	limiters map[string]Limiter
}

// NewDefaultLimiters returns the framework's Limiters implementation.
func NewDefaultLimiters(configProvider *config.Provider) Limiters {
	return &defaultLimiters{
		config:   configProvider,
		limiters: map[string]Limiter{},
	}
}

// Get returns the limiter registered under name, building it on first use.
func (l *defaultLimiters) Get(name string) (Limiter, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	limiter, ok := l.limiters[name]
	if ok {
		return limiter, nil
	}

	cfg, ok := l.config.Get().Limiter.TokenBucket[name]
	if !ok {
		return nil, fmt.Errorf("limiter %q is not defined", name)
	}

	newLimiter := &DefaultLimiter{
		limiter: rate.NewLimiter(rate.Every(cfg.Limit.Duration), cfg.Burst),
	}
	l.limiters[name] = newLimiter

	return newLimiter, nil
}
