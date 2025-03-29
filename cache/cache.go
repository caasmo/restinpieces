package cache

import "time"

// Cache defines a generic interface compatible with Ristretto and other caches
type Cache[K comparable, V any] interface {
	// Get retrieves a value from the cache
	Get(key K) (V, bool)

	// Set stores a value with cost, returning true if successful
	Set(key K, value V, cost int64) bool

	// SetWithTTL stores a value with cost and TTL, returning true if successful
	SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool
}
