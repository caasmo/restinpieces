package core

import (
	"fmt"
	"time"
)

const (
	blockingDuration = 1 * time.Hour // Default blocking duration
	defaultBlockCost = 1             // Default cost for blocked IP entries
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

// BlockIP adds an IP to the blocklist in current and next time bucket with adjusted TTL
func (a *App) BlockIP(ip string) error {
	now := time.Now().Unix()
	currentBucket := now / bucketDurationSec
	nextBucket := currentBucket + 1

	// Calculate remaining time in current bucket
	timeUntilNextBucket := (nextBucket*bucketDurationSec) - now
	ttlCurrent := time.Duration(timeUntilNextBucket) * time.Second

	// Block in current bucket with remaining time
	currentKey := fmt.Sprintf("%s|%d", ip, currentBucket)
	a.cache.SetWithTTL(currentKey, true, defaultBlockCost, ttlCurrent)

	// Block in next bucket with full duration minus what's already passed
	ttlNext := blockingDuration - ttlCurrent
	if ttlNext > 0 {
		nextKey := fmt.Sprintf("%s|%d", ip, nextBucket)
		a.cache.SetWithTTL(nextKey, true, defaultBlockCost, ttlNext)
	}

	return nil
}
