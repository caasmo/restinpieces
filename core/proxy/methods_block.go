package proxy

import (
	"fmt"
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

// IsBlocked checks if an IP is currently blocked
func (px *Proxy) IsBlockeda(ip string) bool {
	currentBucket := getTimeBucket(time.Now())

	// Check current bucket
	if _, found := px.app.Cache().Get(formatBlockKey(ip, currentBucket)); found {
		return true
	}

	return false
}

// BlockIP adds an IP to the blocklist in current and next time bucket with adjusted TTL
func (px *Proxy) BlockIPa(ip string) error {
	now := time.Now()
	currentBucket := getTimeBucket(now)
	nextBucket := currentBucket + 1
	until := now.Add(blockingDuration)

	// Block in current bucket with full blocking duration
	currentKey := formatBlockKey(ip, currentBucket)
	if !px.app.Cache().SetWithTTL(currentKey, true, defaultBlockCost, blockingDuration) {
		px.app.Logger().Error("failed to block IP in current bucket", "ip", ip, "bucket", currentBucket)
		return fmt.Errorf("failed to block IP %s in current bucket %d", ip, currentBucket)
	}
	px.app.Logger().Info("IP blocked in current bucket",
		"ip", ip,
		"bucket", currentBucket,
		"until", until.Format(time.RFC3339))

	// Calculate time until next bucket starts
	nowUnix := now.Unix()
	timeUntilNextBucket := (nextBucket * bucketDurationSec) - nowUnix
	ttlNext := blockingDuration - time.Duration(timeUntilNextBucket)*time.Second

	if ttlNext > 0 {
		nextKey := formatBlockKey(ip, nextBucket)
		if !px.app.Cache().SetWithTTL(nextKey, true, defaultBlockCost, ttlNext) {
			px.app.Logger().Error("failed to block IP in next bucket", "ip", ip, "bucket", nextBucket)
			return fmt.Errorf("failed to block IP %s in next bucket %d", ip, nextBucket)
		}
		px.app.Logger().Info("IP blocked in next bucket",
			"ip", ip,
			"bucket", nextBucket,
			"until", until.Format(time.RFC3339))
	}

	return nil
}
