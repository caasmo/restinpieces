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

// getTimeBucket returns a time bucket string based on current Unix timestamp modulo
func getTimeBucket() string {
    now := time.Now().Unix()
    bucketNum := now / bucketDurationSec
    return fmt.Sprintf("bucket_%d", bucketNum)
}

// TimeBucket is the current time bucket for blocked IP grouping
var TimeBucket = getTimeBucket()

// BlockIP adds an IP to the blocklist with TTL using the app's cache
func (a *App) BlockIP(ip string) error {
	// Create cache key combining IP and time bucket
	key := ip + "|" + TimeBucket
	
	// Store in cache with TTL and default cost
	success := a.cache.SetWithTTL(key, true, defaultBlockCost, blockingDuration)
	if !success {
		return ErrCacheOperationFailed
	}
	return nil
}
