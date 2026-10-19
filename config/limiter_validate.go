package config

import "fmt"

// validateLimiters checks every limiter entry. A token-bucket entry with a
// non-positive limit never refills; a non-positive burst never allows a call.
// A fixed-window entry needs a prefix for its cache keys, a positive window and
// a max of at least 1.
func validateLimiters(limiters Limiter) error {
	if err := validateLimiterLabelsUnique(limiters); err != nil {
		return err
	}
	for name, e := range limiters.TokenBucket {
		if !isValidMapKeyLabel(name) {
			return fmt.Errorf("token_bucket: map key %q must not contain whitespace or '.'", name)
		}
		if e.Limit.Duration <= 0 {
			return fmt.Errorf("limiter.token_bucket.%s.limit must be positive", name)
		}
		if e.Burst <= 0 {
			return fmt.Errorf("limiter.token_bucket.%s.burst must be positive", name)
		}
	}
	for name, e := range limiters.FixedWindow {
		if !isValidMapKeyLabel(name) {
			return fmt.Errorf("fixed_window: map key %q must not contain whitespace or '.'", name)
		}
		if e.Prefix == "" {
			return fmt.Errorf("limiter.fixed_window.%s.prefix is required", name)
		}
		if e.Window.Duration <= 0 {
			return fmt.Errorf("limiter.fixed_window.%s.window must be positive", name)
		}
		if e.Max < 1 {
			return fmt.Errorf("limiter.fixed_window.%s.max must be at least 1", name)
		}
	}
	return nil
}

// validateLimiterLabelsUnique checks that no two limiter entries share a label.
// Every label names exactly one entry, whichever branch it sits in.
func validateLimiterLabelsUnique(limiters Limiter) error {
	labels := make(map[string]struct{}, len(limiters.TokenBucket)+len(limiters.FixedWindow))
	for label := range limiters.TokenBucket {
		labels[label] = struct{}{}
	}
	for label := range limiters.FixedWindow {
		if _, ok := labels[label]; ok {
			return fmt.Errorf("label %q is used by both token_bucket and fixed_window", label)
		}
	}
	return nil
}
