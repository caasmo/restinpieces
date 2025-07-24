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

// New creates a new Ristretto cache instance based on a predefined level.
func New[V any](level string) (cache.Cache[string, V], error) {
	params, ok := cacheLevels[level]
	if !ok {
		// This check is a safeguard; validation in the config should prevent this.
		return nil, fmt.Errorf("invalid cache level provided: %s", level)
	}

	// Instantiate ristretto.NewCache and ristr.Config with string and V
	ristrettoCache, err := ristr.NewCache[string, V](&ristr.Config[string, V]{
		NumCounters: params.NumCounters,
		MaxCost:     params.MaxCost,
		BufferItems: params.BufferItems,
		// Metrics: true, // Enable metrics if needed
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache: %w", err)
	}

	// Return our specialized wrapper Cache[V]
	// which implements cache.Cache[string, V]
	return &Cache[V]{c: ristrettoCache}, nil
}

// CacheParams holds the configuration for a Ristretto cache instance.
type CacheParams struct {
	NumCounters int64
	MaxCost     int64
	BufferItems int64
}

// cacheLevels defines presets for different operational environments,
// mapping semantic VM sizes to Ristretto parameters.
var cacheLevels = map[string]CacheParams{
    "small": {
        NumCounters: 1e5,     // Track 100k keys, assumes ~10k active items
        MaxCost:     1 << 26, // 64MB
        BufferItems: 64,
    },
    "medium": {
        NumCounters: 1e6,     // Track 1M keys, assumes ~100k active items
        MaxCost:     1 << 28, // 256MB
        BufferItems: 128,     // Increase buffer for better batching
    },
    "large": {
        NumCounters: 1e7,     // Track 10M keys, assumes ~1M active items
        MaxCost:     1 << 30, // 1GB
        BufferItems: 256,     // Higher buffer for high-throughput scenarios
    },
    "very-large": {
        NumCounters: 4e7,     // Track 40M keys, assumes ~4M active items
        MaxCost:     1 << 32, // 4GB
        BufferItems: 512,     // Maximum reasonable buffer size
    },
}

