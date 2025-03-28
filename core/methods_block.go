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

// BlockIP adds an IP to the blocklist in current and next time bucket
func (a *App) BlockIP(ip string) error {
	currentBucket := getTimeBucket()
	nextBucket := currentBucket + 1

	// Block in current bucket
	currentKey := fmt.Sprintf("%s|%d", ip, currentBucket)
	a.cache.SetWithTTL(currentKey, true, defaultBlockCost, blockingDuration)

	// Block in next bucket
	nextKey := fmt.Sprintf("%s|%d", ip, nextBucket)
	a.cache.SetWithTTL(nextKey, true, defaultBlockCost, blockingDuration)

	return nil
}
