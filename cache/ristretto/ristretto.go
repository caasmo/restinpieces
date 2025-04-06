package ristretto

import (
	"time"

	"github.com/caasmo/restinpieces/cache"
	ristr "github.com/dgraph-io/ristretto/v2"
)

type Cache[K comparable, V any] struct {
	c *ristr.Cache
}

func (rc *Cache[K, V]) Get(key K) (V, bool) {
	value, found := rc.c.Get(key)
	if !found {
		var zero V
		return zero, false
	}
	return value.(V), true
}

func (rc *Cache[K, V]) Set(key K, value V, cost int64) bool {
	return rc.c.Set(key, value, cost)
}

func (rc *Cache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	// Wait for the item to be processed by the cache
	success := rc.c.SetWithTTL(key, value, cost, ttl)
	return success
}

func New[K comparable, V any]() (cache.Cache[K, V], error) {
	ristretto, err := ristr.NewCache(&ristr.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 30, // maximum cost of cache (1GB)
		BufferItems: 64,      // number of keys per Get buffer
	})
	if err != nil {
		return nil, err
	}

	return &Cache[K, V]{c: ristretto}, nil
}
