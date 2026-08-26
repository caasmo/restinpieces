package restinpieces

import (
	"fmt"
	"sync"
	"time"

	"github.com/caasmo/restinpieces/cache"
)

// node is one slot in the Cache's preallocated array. Its prev and next indexes, together with
// the Cache fields head and tail, implement the LRU list: a doubly-linked
// list that chains all live nodes from most- (head) to least- (tail) recently used.
//
// LRU layout (left to right):
//
//   head (left, MRU) <-> A <-> B <-> C <-> tail (right, LRU)
//      prev <-       -> next
//
// prev points left toward head, next points right toward tail; -1 means no neighbor.
type node[K comparable, V any] struct {
	key        K
	value      V
	expiration int64 // unix nano; 0 = never expires
	prev       int32 // index of previous node toward head (left), -1 = none
	next       int32 // index of next node toward tail (right), -1 = none
}

// Cache is a preallocated, fixed-capacity LRU cache with lazy expiration.
//
// Storage is allocated once at construction and never grows:
//   - nodes is a fixed array of max nodes; prev/next are indexes into it
//   - index maps each key to its node
//   - free holds the indexes of unused nodes
//
// LRU order is a doubly-linked list running left to right:
// head (left, most-recently-used) <-> ... <-> tail (right, least-recently-used).
// Each node's prev points left toward head, next points right toward tail.
//
// Why LRU: the cache never holds more than max nodes. When it is
// full and a new key arrives, an old one must leave. LRU removes the node
// unused for the longest time, betting that anything touched recently will
// be asked for again soon and long-idle keys will not.
//
// Expired nodes are removed lazily on Get. When the cache is full, Set
// evicts the least-recently-used node (the LRU tail, rightmost) to make room.
//
// TODO: proactive reclamation of expired nodes that are never read again
// (rotating cursor sweep, inline on writes) is not yet implemented.
type Cache[K comparable, V any] struct {
	// nodes holds every node, preallocated once by New and never grown.
	// Each node keeps its position in the array for life; only its
	// content changes as it cycles between unused and live.
	nodes []node[K, V]

	// index maps each live key to its node. Presence here is the source
	// of truth for liveness: a node reached through index is live;
	// every other node is free.
	index map[K]int32

	// head is the index of the most-recently-used node (left end of the LRU list), -1 if empty.
	head int32

	// tail is the index of the least-recently-used node (right end of the LRU list), -1 if empty.
	tail int32

	// free holds the indexes of unused nodes. alloc takes one out
	// from the end, dealloc returns it there. len(free) counts what
	// remains; zero means the cache is full.
	free []int32

	// lock guards all fields; every exported method holds it.
	lock sync.Mutex
}

var _ cache.Cache[string, any] = (*Cache[string, any])(nil)

// cacheLevels translates a level string to max, mirroring ristretto's
// presets. Values are based on ristretto's "assumes ~N active items" comments:
// small 10k, medium 100k, large 1M, very-large 4M.
var cacheLevels = map[string]int{
	"small":      10_000,    // ~1 MB
	"medium":     100_000,   // ~10 MB
	"large":      1_000_000, // ~120 MB
	"very-large": 10_000_000, // ~1.1 GB
}

// New creates a cache for string keys based on a predefined level, like ristretto.New.
// It translates the level to max using the same presets as ristretto.
func New[V any](level string) (cache.Cache[string, V], error) {
	max, ok := cacheLevels[level]
	if !ok {
		return nil, fmt.Errorf("invalid cache level provided: %s", level)
	}
	c := &Cache[string, V]{
		nodes: make([]node[string, V], max),
		index: make(map[string]int32, max),
		head:  -1,
		tail:  -1,
		free:  make([]int32, 0, max),
	}
	for n := int32(0); n < int32(max); n++ {
		c.free = append(c.free, n)
	}
	return c, nil
}

// newWithMax creates a cache with max preallocated nodes.
// For tests only; production uses New(level).
func newWithMax[K comparable, V any](max int) *Cache[K, V] {
	c := &Cache[K, V]{
		nodes: make([]node[K, V], max),
		index: make(map[K]int32, max),
		head:  -1,
		tail:  -1,
		free:  make([]int32, 0, max),
	}
	for n := int32(0); n < int32(max); n++ {
		c.free = append(c.free, n)
	}
	return c
}

// Get retrieves the value for key, moving it to the LRU head on a hit.
//
// An expired node is removed lazily on read and reported as a miss.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	n, ok := c.index[key]
	if !ok {
		var zero V
		return zero, false
	}

	// Lazy expiry: time.Now() is a vDSO clock read costing ~40ns, roughly
	// doubling the cost of TTL reads.
	if c.nodes[n].expiration != 0 && time.Now().UnixNano() > c.nodes[n].expiration {
        
		c.unlink(n)
		delete(c.index, key)
		c.dealloc(n)
		var zero V
		return zero, false
	}

	if n == c.head {
		return c.nodes[n].value, true
	}
	c.unlink(n)
	c.linkToHead(n)
	return c.nodes[n].value, true
}

// Set stores key with value and cost. Cost is accepted for interface
// compatibility and currently unused.
func (c *Cache[K, V]) Set(key K, value V, cost int64) bool {
	return c.SetWithTTL(key, value, cost, 0)
}

// SetWithTTL stores key with value, cost and TTL.
//
// A zero TTL means the node never expires. A negative TTL is a no-op and
// returns false, matching ristretto semantics. When the cache is full, the
// least-recently-used node is evicted to make room.
func (c *Cache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	if ttl < 0 {
		return false
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	// TODO: rotating cursor sweep (proactive expiry reclamation) hooks in
	// here, piggybacking on writes; not yet implemented.

	var expiration int64
	if ttl > 0 {
		expiration = time.Now().Add(ttl).UnixNano()
	}

	n, ok := c.index[key]
	if ok {
		// Overwrite: update value and expiration, refresh LRU position.
		c.nodes[n].value = value
		c.nodes[n].expiration = expiration
		if n != c.head {
			c.unlink(n)
			c.linkToHead(n)
		}
		return true
	}

	n = c.alloc()
	if n == -1 && c.tail != -1 {
		// Full: evict the least-recently-used node (tail) to make room.
		nTail := c.tail
		c.unlink(nTail)
		delete(c.index, c.nodes[nTail].key)
		c.dealloc(nTail)
		n = c.alloc()
	}
	if n == -1 {
		// Cannot happen: tail eviction made room when the cache was full.
		return false
	}

	c.nodes[n].key = key
	c.nodes[n].value = value
	c.nodes[n].expiration = expiration
	c.index[key] = n
	c.linkToHead(n)
	return true
}

// alloc returns an unused node index from the right end of free, -1 if none left.
func (c *Cache[K, V]) alloc() int32 {
	len := len(c.free)
	if len == 0 {
		return -1
	}
	n := c.free[len-1]
	c.free = c.free[:len-1]
	return n
}

// dealloc returns node index n to the right end of free.
func (c *Cache[K, V]) dealloc(n int32) {
	c.free = append(c.free, n)
}

// unlink removes node n from the LRU list.
//
// unlink(B) does this:
// 1. Look at B's neighbors: prev = B.prev (A), next = B.next (C)
// 2. Stitch them together: A.next = C and C.prev = A -> now A <-> C
// 3. If B was at an end, update the end pointer:
// - B was head (prev == -1) -> head = C
// - B was tail (next == -1) -> tail = A
// 4. Isolate B: B.prev = -1, B.next = -1
//
// Result: B is floating alone, the rest of the line is unbroken.
//
// Before: head -> A <-> B <-> C -> tail
// After unlink(B): head -> A <-> C -> tail    B: -1 - 1
//
// Edge cases handled by same code:
// - unlink(head) -> next node becomes new head
// - unlink(tail) -> prev node becomes new tail
// - unlink only node -> head = -1, tail = -1 (empty list)
func (c *Cache[K, V]) unlink(n int32) {
	// 1. Look at B's neighbors: prev = B.prev (A), next = B.next (C)
	prev, next := c.nodes[n].prev, c.nodes[n].next
	// 2. Stitch them together: A.next = C and C.prev = A -> now A <-> C
	// 3. If B was at an end, update the end pointer:
	if prev != -1 {
		c.nodes[prev].next = next
	} else {
		// - B was head (prev == -1) -> head = C
		c.head = next
	}
	if next != -1 {
		c.nodes[next].prev = prev
	} else {
		// - B was tail (next == -1) -> tail = A
		c.tail = prev
	}
	// 4. Isolate B: B.prev = -1, B.next = -1
	c.nodes[n].prev = -1
	c.nodes[n].next = -1
}

// linkToHead links node n as the LRU head (most-recently-used, left end).
//
// linkToHead(N) does this:
// 1. Prepare N: N.prev = -1 (no left), N.next = head (old head A)
// 2. Stitch old head back to N: if head != -1, A.prev = N -> now N <-> A
// 3. Move head to N: head = N
// 4. If list was empty (tail == -1), tail = N
//
// Result: N is new head, former list follows to its right.
//
// Before (non-empty): head -> A <-> B -> tail
// After linkToHead(N): head -> N <-> A <-> B -> tail
// Before (empty): head = -1, tail = -1
// After linkToHead(N) on empty: head -> N -> tail   N: -1 - -1
//
// Edge cases handled by same code:
// - linkToHead on empty list -> head = N, tail = N
// - linkToHead on non-empty list -> old head's prev = N
func (c *Cache[K, V]) linkToHead(n int32) {
	// 1. Prepare N: N.prev = -1 (no left), N.next = head (old head A)
	c.nodes[n].prev = -1
	c.nodes[n].next = c.head
	// 2. Stitch old head back to N: if head != -1, A.prev = N -> now N <-> A
	if c.head != -1 {
		c.nodes[c.head].prev = n
	}
	// 3. Move head to N: head = N
	c.head = n
	// 4. If list was empty (tail == -1), tail = N
	if c.tail == -1 {
		c.tail = n
	}
}


