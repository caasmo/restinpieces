package ristretto

import (
	"testing"
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
