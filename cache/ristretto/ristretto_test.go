package ristretto

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Parallel()

	validLevels := []string{"small", "medium", "large", "very-large"}
	for _, level := range validLevels {
		t.Run(level, func(t *testing.T) {
			cache, err := New[any](level)
			if err != nil {
				t.Errorf("New(%q) returned an unexpected error: %v", level, err)
			}
			if cache == nil {
				t.Errorf("New(%q) returned a nil cache, but no error", level)
			}
		})
	}

	invalidLevels := []string{"", "invalid-level", " medium"}
	for _, level := range invalidLevels {
		t.Run(level, func(t *testing.T) {
			cache, err := New[any](level)
			if err == nil {
				t.Errorf("New(%q) was expected to return an error, but did not", level)
			}
			if cache != nil {
				t.Errorf("New(%q) was expected to return a nil cache, but did not", level)
			}
		})
	}
}

func TestCache_SetAndGet(t *testing.T) {
	t.Parallel()
	cache, err := New[string]("small")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// 1. Basic Set and Get
	key, value := "test-key", "test-value"
	cache.Set(key, value, 1)
	// Ristretto processes writes asynchronously, so a small delay is needed for the value to become available.
	time.Sleep(10 * time.Millisecond)

	retrieved, found := cache.Get(key)
	if !found {
		t.Errorf("expected to find key %q, but it was not found", key)
	}
	if retrieved != value {
		t.Errorf("expected value %q, but got %q", value, retrieved)
	}

	// 2. Get Non-Existent Key
	retrieved, found = cache.Get("non-existent-key")
	if found {
		t.Error("expected not to find key, but it was found")
	}
	if retrieved != "" {
		t.Errorf("expected zero value \"\", but got %q", retrieved)
	}

	// 3. Overwrite Key
	newValue := "new-value"
	cache.Set(key, newValue, 1)
	time.Sleep(10 * time.Millisecond)

	retrieved, found = cache.Get(key)
	if !found {
		t.Errorf("expected to find key %q after overwrite, but it was not found", key)
	}
	if retrieved != newValue {
		t.Errorf("expected overwritten value %q, but got %q", newValue, retrieved)
	}
}

func TestCache_SetWithTTL(t *testing.T) {
	t.Parallel()
	cache, err := New[int]("small")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	key, value := "ttl-key", 123
	ttl := 20 * time.Millisecond

	cache.SetWithTTL(key, value, 1, ttl)
	time.Sleep(10 * time.Millisecond) // Wait for write to process

	// 1. Check that the key is present before expiration
	retrieved, found := cache.Get(key)
	if !found {
		t.Fatal("key not found before TTL expiration")
	}
	if retrieved != value {
		t.Fatalf("expected value %d, but got %d", value, retrieved)
	}

	// 2. Wait for the TTL to expire
	time.Sleep(ttl)

	// 3. Check that the key is gone after expiration
	retrieved, found = cache.Get(key)
	if found {
		t.Errorf("key was found after TTL expiration, but should have been evicted")
	}
	if retrieved != 0 {
		t.Errorf("expected zero value 0 for int, but got %d", retrieved)
	}
}

func TestCache_ZeroValue(t *testing.T) {
	t.Parallel()

	t.Run("string", func(t *testing.T) {
		cache, _ := New[string]("small")
		val, found := cache.Get("key")
		if found || val != "" {
			t.Errorf(`expected (\"\", false), got (%q, %v)`, val, found)
		}
	})

	t.Run("int", func(t *testing.T) {
		cache, _ := New[int]("small")
		val, found := cache.Get("key")
		if found || val != 0 {
			t.Errorf(`expected (0, false), got (%d, %v)`, val, found)
		}
	})

	type testStruct struct{ A int }
	t.Run("struct", func(t *testing.T) {
		cache, _ := New[testStruct]("small")
		val, found := cache.Get("key")
		if found || val != (testStruct{}) {
			t.Errorf(`expected ({}, false), got (%v, %v)`, val, found)
		}
	})

	t.Run("pointer", func(t *testing.T) {
		cache, _ := New[*testStruct]("small")
		val, found := cache.Get("key")
		if found || val != nil {
			t.Errorf(`expected (nil, false), got (%v, %v)`, val, found)
		}
	})
}
