package cache

import (
	"errors"
	"sync"
	"testing"
)

func TestCache_IncrementCreatesCounter(t *testing.T) {
	c := newWith[string, any](10)

	count, err := c.Increment("key", 2)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("Increment() = %d, want 2", count)
	}

	got, ok := c.Get("key")
	if !ok {
		t.Fatal("expected the counter to be stored")
	}
	if got != uint64(2) {
		t.Fatalf("Get() = %v, want 2", got)
	}
}

func TestCache_IncrementCreatesWithoutExpiration(t *testing.T) {
	c := newWith[string, uint64](2)

	_, err := c.Increment("key", 1)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}

	n := c.index["key"]
	if c.nodes[n].expiration != 0 {
		t.Fatalf("expiration = %d, want 0", c.nodes[n].expiration)
	}
}

func TestCache_IncrementAddsToCounter(t *testing.T) {
	c := newWith[string, any](10)

	_, _ = c.Increment("key", 2)
	count, err := c.Increment("key", 3)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if count != 5 {
		t.Fatalf("Increment() = %d, want 5", count)
	}
}

func TestCache_IncrementTypedCounter(t *testing.T) {
	c := newWith[string, uint64](10)

	count, err := c.Increment("key", 4)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if count != 4 {
		t.Fatalf("Increment() = %d, want 4", count)
	}
}

func TestCache_IncrementRejectsNonCounter(t *testing.T) {
	c := newWith[string, any](10)
	c.Set("key", true, 1)

	count, err := c.Increment("key", 1)
	if !errors.Is(err, ErrNotCounter) {
		t.Fatalf("Increment() error = %v, want %v", err, ErrNotCounter)
	}
	if count != 0 {
		t.Fatalf("Increment() = %d, want 0", count)
	}

	got, ok := c.Get("key")
	if !ok || got != true {
		t.Fatalf("Get() = (%v, %v), want (true, true)", got, ok)
	}
}

func TestCache_IncrementRejectsWrongValueType(t *testing.T) {
	c := newWith[string, string](2)

	count, err := c.Increment("key", 1)
	if !errors.Is(err, ErrNotCounter) {
		t.Fatalf("Increment() error = %v, want %v", err, ErrNotCounter)
	}
	if count != 0 {
		t.Fatalf("Increment() = %d, want 0", count)
	}

	if _, ok := c.Get("key"); ok {
		t.Fatal("expected no entry to be stored")
	}
	if len(c.free) != 2 {
		t.Fatalf("len(free) = %d, want 2", len(c.free))
	}
}

func TestCache_IncrementZeroCapacity(t *testing.T) {
	c := newWith[string, uint64](0)

	count, err := c.Increment("key", 1)
	if !errors.Is(err, errNoFreeNode) {
		t.Fatalf("Increment() error = %v, want %v", err, errNoFreeNode)
	}
	if count != 0 {
		t.Fatalf("Increment() = %d, want 0", count)
	}
	if _, ok := c.Get("key"); ok {
		t.Fatal("expected no entry to be stored")
	}
}

func TestCache_IncrementZeroDeltaCreatesCounter(t *testing.T) {
	c := newWith[string, uint64](2)

	_, _ = c.Increment("a", 1)
	_, _ = c.Increment("b", 1)

	count, err := c.Increment("c", 0)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("Increment() = %d, want 0", count)
	}

	if _, ok := c.Get("a"); ok {
		t.Fatal("expected a to be evicted")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("expected c to be present")
	}
}

func TestCache_IncrementWrapsOnOverflow(t *testing.T) {
	c := newWith[string, uint64](2)

	_, err := c.Increment("key", ^uint64(0))
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}

	count, err := c.Increment("key", 1)
	if err != nil {
		t.Fatalf("Increment() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("Increment() = %d, want 0", count)
	}
}

func TestCache_IncrementEvictsLRUTail(t *testing.T) {
	c := newWith[string, any](2)

	_, _ = c.Increment("a", 1)
	_, _ = c.Increment("b", 1)
	_, _ = c.Increment("c", 1)

	if _, ok := c.Get("a"); ok {
		t.Fatal("expected a to be evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("expected b to be present")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("expected c to be present")
	}
}

func TestCache_IncrementRefreshesLRU(t *testing.T) {
	c := newWith[string, any](3)

	_, _ = c.Increment("a", 1)
	_, _ = c.Increment("b", 1)
	_, _ = c.Increment("c", 1)
	_, _ = c.Increment("a", 1)
	_, _ = c.Increment("d", 1)

	if _, ok := c.Get("b"); ok {
		t.Fatal("expected b to be evicted")
	}
	for _, key := range []string{"a", "c", "d"} {
		if _, ok := c.Get(key); !ok {
			t.Fatalf("expected %s to be present", key)
		}
	}
}

func TestCache_IncrementConcurrent(t *testing.T) {
	c := newWith[string, any](10)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				_, err := c.Increment("key", 1)
				if err != nil {
					t.Errorf("Increment() error = %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got, ok := c.Get("key")
	if !ok {
		t.Fatal("expected the counter to be present")
	}
	if got != uint64(8*500) {
		t.Fatalf("Get() = %v, want %d", got, 8*500)
	}
}
