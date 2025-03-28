package ristretto

import (
	"time"

	"github.com/caasmo/restinpieces/cache"
	ristr "github.com/outcaste-io/ristretto"
)

type Cache[K comparable, V any] struct {
	c *ristr.Cache
}

func (c *Cache[K, V]) Del(key K) {
	c.c.Del(key)
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	value, found := c.c.Get(key)
	if !found {
		var zero V
		return zero, false
	}
	return value.(V), true
}

func (c *Cache[K, V]) GetTTL(key K) (time.Duration, bool) {
	// Ristretto doesn't expose TTL directly, so we return 0/false
	// Alternatively could implement custom TTL tracking
	return 0, false
}

func (c *Cache[K, V]) MaxCost() int64 {
	return c.c.MaxCost()
}

func (c *Cache[K, V]) Set(key K, value V, cost int64) bool {
	return c.c.Set(key, value, cost)
}

func (c *Cache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	// Ristretto doesn't support per-item TTL, so we ignore it
	return c.c.Set(key, value, cost)
}

func New[K comparable, V any]() (cache.Cache[K, V], error) {
	c, err := ristr.NewCache(&ristr.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 30, // maximum cost of cache (1GB)
		BufferItems: 64,      // number of keys per Get buffer
	})
	if err != nil {
		return nil, err
	}

	return &Cache[K, V]{c: c}, nil
}
