package core

import (
	"fmt"
	"log/slog"
	"time"
)

const (
	//blockingDuration = 1 * time.Hour // Default blocking duration
	blockingDuration = 3 * time.Minute // Default blocking duration
	defaultBlockCost = 1               // Default cost for blocked IP entries
)

const (
	bucketDurationSec = 3600 // 1 hour buckets
)

// getTimeBucket returns the bucket number for a given time (periods since Unix epoch)
func getTimeBucket(t time.Time) int64 {
	return t.Unix() / bucketDurationSec
}

// formatBlockKey creates a consistent cache key for blocked IPs
func formatBlockKey(ip string, bucket int64) string {
	return fmt.Sprintf("%s|%d", ip, bucket)
}

// IsBlocked checks if an IP is currently blocked in any relevant time bucket
func (a *App) IsBlocked(ip string) bool {
	currentBucket := getTimeBucket(time.Now())

	// Check current bucket
	if _, found := a.cache.Get(formatBlockKey(ip, currentBucket)); found {
		return true
	}

	// Check next bucket
	//if _, found := a.cache.Get(formatBlockKey(ip, currentBucket+1)); found {
	//	return true
	//}

	return false
}

// BlockIP adds an IP to the blocklist in current and next time bucket with adjusted TTL
func (a *App) BlockIP(ip string) error {
	now := time.Now()
	currentBucket := getTimeBucket(now)
	nextBucket := currentBucket + 1

	// Block in current bucket with full blocking duration
	currentKey := formatBlockKey(ip, currentBucket)
	successCurrent := a.cache.SetWithTTL(currentKey, true, defaultBlockCost, blockingDuration)
	until := now.Add(blockingDuration)
	slog.Info("IP blocked in current bucket",
		"ip", ip,
		"bucket", currentBucket,
		"until", until.Format(time.RFC3339),
		"success", successCurrent)

	// Calculate time until next bucket starts
	nowUnix := now.Unix()
	timeUntilNextBucket := (nextBucket * bucketDurationSec) - nowUnix
	ttlNext := blockingDuration - time.Duration(timeUntilNextBucket)*time.Second
	if ttlNext > 0 {
		nextKey := formatBlockKey(ip, nextBucket)
		successNext := a.cache.SetWithTTL(nextKey, true, defaultBlockCost, ttlNext)
		until := now.Add(blockingDuration)
		slog.Info("IP blocked in next bucket",
			"ip", ip,
			"bucket", nextBucket,
			"until", until.Format(time.RFC3339),
			"success", successNext)
	}

	return nil
}
