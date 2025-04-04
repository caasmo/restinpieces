package proxy

import (
	"fmt"
	"time"

	"github.com/caasmo/restinpieces/cache"
	// "github.com/caasmo/restinpieces/config" // No longer needed here
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

// BlockIp implements the FeatureBlocker interface using a cache for storage.
type BlockIp struct {
	cache cache.Cache[string, interface{}]
}

// NewBlockIp creates a new BlockIp instance with the given cache.
func NewBlockIp(cache cache.Cache[string, interface{}]) *BlockIp {
	return &BlockIp{
		cache: cache,
	}
}

// IsEnabled checks if the IP blocking feature is enabled based on configuration.
// Placeholder implementation: always returns true.
func (b *BlockIp) IsEnabled() bool {
	// TODO: Implement actual logic based on b.config
	return true
}

// IsBlocked checks if a given IP address is currently blocked by looking in the cache.
func (b *BlockIp) IsBlocked(ip string) bool {
	currentBucket := getTimeBucket(time.Now())
	key := formatBlockKey(ip, currentBucket)
	_, found := b.cache.Get(key)
	return found
}

// Block adds the given IP to the block list.
// Placeholder implementation: does nothing yet.
func (b *BlockIp) Block(ip string) error {
	now := time.Now()
	currentBucket := getTimeBucket(now)
	nextBucket := currentBucket + 1
	until := now.Add(blockingDuration)

	// Block in current bucket with full blocking duration
	currentKey := formatBlockKey(ip, currentBucket)
	// Use the internal cache instance (b.cache)
	// Removed logging as BlockIp doesn't have a logger instance
	if !b.cache.SetWithTTL(currentKey, true, defaultBlockCost, blockingDuration) {
		// px.app.Logger().Error("failed to block IP in current bucket", "ip", ip, "bucket", currentBucket)
		return fmt.Errorf("failed to block IP %s in current bucket %d", ip, currentBucket)
	}
	// px.app.Logger().Info("IP blocked in current bucket",
	// 	"ip", ip,
	// 	"bucket", currentBucket,
	// 	"until", until.Format(time.RFC3339))

	// Calculate time until next bucket starts
	nowUnix := now.Unix()
	timeUntilNextBucket := (nextBucket * bucketDurationSec) - nowUnix
	ttlNext := blockingDuration - time.Duration(timeUntilNextBucket)*time.Second

	if ttlNext > 0 {
		nextKey := formatBlockKey(ip, nextBucket)
		// Use the internal cache instance (b.cache)
		// Removed logging as BlockIp doesn't have a logger instance
		if !b.cache.SetWithTTL(nextKey, true, defaultBlockCost, ttlNext) {
			// px.app.Logger().Error("failed to block IP in next bucket", "ip", ip, "bucket", nextBucket)
			return fmt.Errorf("failed to block IP %s in next bucket %d", ip, nextBucket)
		}
		// px.app.Logger().Info("IP blocked in next bucket",
		// 	"ip", ip,
		// 	"bucket", nextBucket,
		// 	"until", until.Format(time.RFC3339))
	}

	return nil

}

// Block for DisabledBlock does nothing and returns nil.
func (d *DisabledBlock) Block(ip string) error {
	return nil // Blocking is disabled
}

// DisabledBlock implements the FeatureBlocker interface but always returns false,
// effectively disabling the blocking feature.
type DisabledBlock struct{}

// IsEnabled always returns false, indicating the feature is disabled.
func (d *DisabledBlock) IsEnabled() bool {
	return false
}

// IsBlocked always returns false, indicating no IP is ever blocked.
func (d *DisabledBlock) IsBlocked(ip string) bool {
	return false
}
