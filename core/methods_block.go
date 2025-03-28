package core

import (
	"fmt"
	"log/slog"
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

// IsBlocked checks if an IP is currently blocked in any relevant time bucket
func (a *App) IsBlocked(ip string) bool {
    currentBucket := getTimeBucket()
    
    // Check current bucket
    if _, found := a.cache.Get(FormatBlockKey(ip, currentBucket)); found {
        return true
    }
    
    // Check next bucket
    if _, found := a.cache.Get(FormatBlockKey(ip, currentBucket+1)); found {
        return true
    }
    
    return false
}

// BlockIP adds an IP to the blocklist in current and next time bucket with adjusted TTL
func (a *App) BlockIP(ip string) error {
    now := time.Now().Unix()
    currentBucket := now / bucketDurationSec
    nextBucket := currentBucket + 1

    // Calculate remaining time in current bucket
    timeUntilNextBucket := (nextBucket*bucketDurationSec) - now
    ttlCurrent := time.Duration(timeUntilNextBucket) * time.Second

    // Block in current bucket with remaining time
    currentKey := FormatBlockKey(ip, currentBucket)
    successCurrent := a.cache.SetWithTTL(currentKey, true, defaultBlockCost, ttlCurrent)
    slog.Info("IP blocked in current bucket",
        "ip", ip,
        "bucket", currentBucket,
        "ttl", ttlCurrent,
        "success", successCurrent)

    // Block in next bucket with full duration minus what's already passed
    ttlNext := blockingDuration - ttlCurrent
    if ttlNext > 0 {
        nextKey := FormatBlockKey(ip, nextBucket)
        successNext := a.cache.SetWithTTL(nextKey, true, defaultBlockCost, ttlNext)
        slog.Info("IP blocked in next bucket",
            "ip", ip,
            "bucket", nextBucket,
            "ttl", ttlNext,
            "success", successNext)
    }

    return nil
}
