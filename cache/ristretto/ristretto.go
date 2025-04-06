package ristretto

import (
	"fmt"
	"time"

	"github.com/caasmo/restinpieces/cache"
	// https://pkg.go.dev/github.com/dgraph-io/ristretto/v2
	ristr "github.com/dgraph-io/ristretto/v2"
)

// Cache wrapper specialized for string keys.
// It remains generic over the value type V.
type Cache[V any] struct {
	// Instantiate ristr.Cache with string as the key type
	c *ristr.Cache[string, V]
}

// Ensure our specialized Cache implements the generic cache.Cache interface
// for string keys.
var _ cache.Cache[string, any] = (*Cache[any])(nil)

// Get retrieves a value using a string key.
func (rc *Cache[V]) Get(key string) (V, bool) {
	// Assuming ristretto.Cache[string, V].Get returns V directly as per user request.
	value, found := rc.c.Get(key)
	if !found {
		var zero V
		return zero, false
	}
	// No type assertion needed if Get returns V directly.
	return value, true
}

// Set stores a value with a string key.
func (rc *Cache[V]) Set(key string, value V, cost int64) bool {
	return rc.c.Set(key, value, cost)
}

// SetWithTTL stores a value with a string key and TTL.
func (rc *Cache[V]) SetWithTTL(key string, value V, cost int64, ttl time.Duration) bool {
	return rc.c.SetWithTTL(key, value, cost, ttl)
}

// New creates a new Ristretto cache instance specialized for string keys
// and generic for the value type V.
func New[V any]() (cache.Cache[string, V], error) {
	// Instantiate ristretto.NewCache and ristr.Config with string and V
	ristrettoCache, err := ristr.NewCache[string, V](&ristr.Config[string, V]{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 30, // maximum cost of cache (1GB)
		BufferItems: 64,      // number of keys per Get buffer
		// Metrics: true, // Enable metrics if needed
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache: %w", err)
	}

	// Return our specialized wrapper Cache[V]
	// which implements cache.Cache[string, V]
	return &Cache[V]{c: ristrettoCache}, nil
}
