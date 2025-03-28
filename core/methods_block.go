package core

import (
	"time"
)

const (
	blockingDuration = 1 * time.Hour // Default blocking duration
	defaultBlockCost = 1             // Default cost for blocked IP entries
)

const (
    bucketDurationSec = 3600 // 1 hour buckets
)

const (
    bucketDurationSec = 3600 // 1 hour buckets
)

// getTimeBucket returns the current bucket number (periods since Unix epoch)
func getTimeBucket() int64 {
    return time.Now().Unix() / bucketDurationSec
}

// TimeBucket is the current time bucket number for blocked IP grouping
var TimeBucket = getTimeBucket()

// BlockIP adds an IP to the blocklist with TTL using the app's cache
func (a *App) BlockIP(ip string) error {
	// Create cache key combining IP and time bucket number
	key := fmt.Sprintf("%s|%d", ip, TimeBucket)
	
	// Store in cache with TTL and default cost
	success := a.cache.SetWithTTL(key, true, defaultBlockCost, blockingDuration)
	if !success {
		return ErrCacheOperationFailed
	}
	return nil
}
