package cache

import "errors"

// ErrNotCounter reports that a cache value is not a uint64 counter.
var ErrNotCounter = errors.New("not a counter")

// errNoFreeNode reports that the cache has no node to reserve.
var errNoFreeNode = errors.New("cache has no free node")

// Increment raises the counter at key by delta in one locked move.
// A key not yet counted starts at delta and never expires.
// An expired node is removed and replaced with a new counter. The
// replacement does not inherit the old TTL.
// An existing live key keeps its expiration. delta is added as given; a
// zero delta on a missing key creates a zero counter and may evict the tail.
// Counts wrap on overflow.
// It returns ErrNotCounter when the value type cannot hold a uint64
// or when the stored value is not a uint64 counter. It returns
// errNoFreeNode when the cache has no free node.
func (c *Default[K, V]) Increment(key K, delta uint64) (uint64, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	n, ok := c.index[key]
	var nd *node[K, V]

	// An expired entry behaves like a miss.
	expired := false
	if ok {
		nd = &c.nodes[n]
		// A zero expiration means the entry never expires, so the clock
		// is skipped for counters that have no TTL.
		expired = nd.expiration != 0 && fastNow() > nd.expiration
		if expired {
			ok = false
		}
	}

	if !ok {
		// Check the value type before reserving a node. A failed
		// assertion after alloc would leak the reserved node.
		first, canStore := any(delta).(V)
		if !canStore {
			return 0, ErrNotCounter
		}

		if expired {
			// Remove the dead entry only after the type check, so a
			// failed check leaves the cache unchanged.
			c.unlink(n)
			delete(c.index, key)
			c.dealloc(n)
		}

		// New key: reserve a node for its first count.
		n = c.alloc()
		if n == -1 && c.tail != -1 {
			// The cache is full. Drop the least-recently-used node
			// (the tail) and reuse its slot.
			nTail := c.tail
			c.unlink(nTail)
			delete(c.index, c.nodes[nTail].key)
			c.dealloc(nTail)
			n = c.alloc()
		}
		if n == -1 {
			// A zero-capacity cache has no node to reserve.
			return 0, errNoFreeNode
		}

		nd = &c.nodes[n]
		// Store the first count and make the node the most recently used.
		nd.key = key
		nd.value = first
		nd.expiration = 0
		c.index[key] = n
		c.linkToHead(n)

		return delta, nil
	}

	// Existing key: it must hold a counter.
	current, ok := any(nd.value).(uint64)
	if !ok {
		return 0, ErrNotCounter
	}

	// Add delta, store the new count, and make the node the most recently used.
	count := current + delta
	nd.value = any(count).(V)
	if n != c.head {
		c.unlink(n)
		c.linkToHead(n)
	}

	return count, nil
}
