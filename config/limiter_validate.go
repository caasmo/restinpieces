package config

import "fmt"

// validateLimiters checks every limiter entry. A limiter with a non-positive
// limit never refills; a non-positive burst never allows a call.
func validateLimiters(limiters Limiter) error {
	for name, e := range limiters {
		if e.Limit.Duration <= 0 {
			return fmt.Errorf("limiter.%s.limit must be positive", name)
		}
		if e.Burst <= 0 {
			return fmt.Errorf("limiter.%s.burst must be positive", name)
		}
	}
	return nil
}
