package ristretto

import (
	"time"

	"github.com/caasmo/restinpieces/cache"
	"github.com/dgraph-io/ristretto/v2"
)

type Cache[K comparable, V any] struct {
	cache *ristretto.Cache
}

func (rc *Cache[K, V]) Get(key K) (V, bool) {
	value, found := rc.cache.Get(key)
	if !found {
		var zero V
		return zero, false
	}
	return value.(V), true
}

func (rc *Cache[K, V]) Set(key K, value V, cost int64) bool {
	return rc.cache.Set(key, value, cost)
}

func (rc *Cache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	// Wait for the item to be processed by the cache
	success := rc.cache.SetWithTTL(key, value, cost, ttl)
	return success
}

func New[K comparable, V any]() (cache.Cache[K, V], error) {
	c, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 30, // maximum cost of cache (1GB)
		BufferItems: 64,      // number of keys per Get buffer
	})
	if err != nil {
		return nil, err
	}

	return &Cache[K, V]{cache: c}, nil
}
