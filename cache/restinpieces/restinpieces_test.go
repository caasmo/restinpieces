package restinpieces

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	c := newWith[string, string](10)

	c.Set("key", "value", 1)
	got, ok := c.Get("key")
	if !ok || got != "value" {
		t.Fatalf("expected (value, true), got (%q, %v)", got, ok)
	}

	if _, ok := c.Get("missing"); ok {
		t.Fatal("expected a miss for a missing key")
	}
}

func TestCache_Overwrite(t *testing.T) {
	c := newWith[string, string](10)

	c.Set("key", "old", 1)
	c.Set("key", "new", 1)

	got, ok := c.Get("key")
	if !ok || got != "new" {
		t.Fatalf("expected (new, true), got (%q, %v)", got, ok)
	}
}

func TestCache_SetWithTTL(t *testing.T) {
	c := newWith[string, string](10)
	ttl := 20 * time.Millisecond

	c.SetWithTTL("key", "value", 1, ttl)

	got, ok := c.Get("key")
	if !ok || got != "value" {
		t.Fatalf("expected key to be present before expiry, got (%q, %v)", got, ok)
	}

	time.Sleep(ttl * 2)

	if _, ok := c.Get("key"); ok {
		t.Fatal("expected key to be expired")
	}
}

func TestCache_SetWithTTLNegativeIsNoop(t *testing.T) {
	c := newWith[string, string](10)

	if c.SetWithTTL("key", "value", 1, -1*time.Second) {
		t.Fatal("expected SetWithTTL with a negative TTL to return false")
	}
	if _, ok := c.Get("key"); ok {
		t.Fatal("expected no entry to be stored for a negative TTL")
	}
}

func TestCache_SetWithTTLZeroNeverExpires(t *testing.T) {
	c := newWith[string, string](10)

	c.SetWithTTL("key", "value", 1, 0)
	time.Sleep(20 * time.Millisecond)

	if _, ok := c.Get("key"); !ok {
		t.Fatal("expected a zero-TTL entry to never expire")
	}
}

func TestCache_Eviction(t *testing.T) {
	c := newWith[string, string](3)

	c.Set("a", "1", 1)
	c.Set("b", "2", 1)
	c.Set("c", "3", 1)
	c.Set("d", "4", 1) // evicts "a", the least-recently-used entry

	if _, ok := c.Get("a"); ok {
		t.Fatal("expected a to be evicted")
	}
	for _, k := range []string{"b", "c", "d"} {
		if _, ok := c.Get(k); !ok {
			t.Fatalf("expected %s to be present", k)
		}
	}
}

func TestCache_GetRefreshesLRU(t *testing.T) {
	c := newWith[string, string](3)

	c.Set("a", "1", 1)
	c.Set("b", "2", 1)
	c.Set("c", "3", 1)
	c.Get("a") // move a to head -> b becomes the tail
	c.Set("d", "4", 1)

	if _, ok := c.Get("b"); ok {
		t.Fatal("expected b to be evicted")
	}
	for _, k := range []string{"a", "c", "d"} {
		if _, ok := c.Get(k); !ok {
			t.Fatalf("expected %s to be present", k)
		}
	}
}

func TestCache_OverwriteRefreshesLRU(t *testing.T) {
	c := newWith[string, string](3)

	c.Set("a", "1", 1)
	c.Set("b", "2", 1)
	c.Set("c", "3", 1)
	c.Set("a", "1", 1) // overwrite moves a to head -> b becomes the tail
	c.Set("d", "4", 1)

	if _, ok := c.Get("b"); ok {
		t.Fatal("expected b to be evicted")
	}
	for _, k := range []string{"a", "c", "d"} {
		if _, ok := c.Get(k); !ok {
			t.Fatalf("expected %s to be present", k)
		}
	}
}

func TestCache_LazyExpiryMakesRoom(t *testing.T) {
	c := newWith[string, string](2)
	ttl := 20 * time.Millisecond

	c.SetWithTTL("a", "1", 1, ttl)
	c.Set("b", "2", 1)
	time.Sleep(ttl * 2)

	if _, ok := c.Get("a"); ok {
		t.Fatal("expected a to be expired")
	}

	// The room freed by lazy expiry must be reusable without evicting b.
	c.Set("c", "3", 1)
	if _, ok := c.Get("b"); !ok {
		t.Fatal("expected b to survive")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("expected c to be present")
	}
}

func TestCache_ReinsertAfterEviction(t *testing.T) {
	c := newWith[string, string](2)

	c.Set("a", "1", 1)
	c.Set("b", "2", 1)
	c.Set("c", "3", 1) // evicts a
	c.Set("a", "1", 1) // re-inserts a, evicts b

	if _, ok := c.Get("b"); ok {
		t.Fatal("expected b to be evicted")
	}
	for _, k := range []string{"a", "c"} {
		if _, ok := c.Get(k); !ok {
			t.Fatalf("expected %s to be present", k)
		}
	}
}

func TestCache_ZeroValueOnMiss(t *testing.T) {
	c := newWith[string, int](10)

	v, ok := c.Get("missing")
	if ok || v != 0 {
		t.Fatalf("expected (0, false), got (%d, %v)", v, ok)
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := newWith[string, int](100)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				key := fmt.Sprintf("k%d", i%50)
				c.Set(key, i, 1)
				c.Get(key)
			}
		}()
	}
	wg.Wait()
}
