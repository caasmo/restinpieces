package cache

package cache

import "time"

// Cache defines a generic interface compatible with Ristretto and other caches
type Cache[K comparable, V any] interface {
	// Del removes a key from the cache
	Del(key K)

	// Get retrieves a value from the cache
	Get(key K) (V, bool)

	// GetTTL returns the remaining time-to-live for a key
	GetTTL(key K) (time.Duration, bool)

	// MaxCost returns the maximum cache capacity in cost units
	MaxCost() int64

	// Set stores a value with cost, returning true if successful
	Set(key K, value V, cost int64) bool

	// SetWithTTL stores a value with cost and TTL, returning true if successful
	SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool
}
