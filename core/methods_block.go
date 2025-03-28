package core

import (
	"time"
)

const (
	blockingDuration = 1 * time.Hour // Default blocking duration
)

// TimeBucket will be used to group blocked IPs by time windows
// This will be defined when we implement the time bucketing logic
var TimeBucket string

// BlockIP adds an IP to the blocklist with TTL using the app's cache
func (a *App) BlockIP(ip string) error {
	// Create cache key combining IP and time bucket
	key := ip + "|" + TimeBucket
	
	// Store in cache with TTL
	success := a.cache.SetWithTTL(key, true, 0, blockingDuration)
	if !success {
		return ErrCacheOperationFailed
	}
	return nil
}
