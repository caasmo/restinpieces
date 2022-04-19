package ristretto

import (
	"github.com/caasmo/restinpieces/cache"
	ristr "github.com/outcaste-io/ristretto"
)

// Implementation of the cache interface (incomplete)
type Cache struct {
	C *ristr.Cache
}

func (c *Cache) Get(key interface{}) (interface{}, bool) {
	return c.C.Get(key)
}

func (c *Cache) Set(key, value interface{}, cost int64) bool {
	return c.C.Set(key, value, cost)
}

func New() (cache.Cache, error) {
	c, err := ristr.NewCache(&ristr.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
	})

	if err != nil {
		return nil, err
	}

	return &Cache{C: c}, nil
}
