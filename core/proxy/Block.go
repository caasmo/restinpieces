package proxy

import (
	"fmt"
	"time"

	"github.com/caasmo/restinpieces/cache"
)

const (
	// Duration for which an IP remains blocked (e.g., 1 hour)
	// TODO: Make this configurable via config.Config
	blockDuration     = 1 * time.Hour
	bucketDurationSec = int64(blockDuration / time.Second)
)

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

// IsEnabled indicates that if this blocker is in use, the feature is considered enabled.
// The decision to use this blocker vs DisabledBlock is made during Proxy initialization based on config.
func (b *BlockIp) IsEnabled() bool {
	return true
}

// IsBlocked checks if a given IP address is currently blocked by looking in the cache.
func (b *BlockIp) IsBlocked(ip string) bool {
	currentBucket := getTimeBucket(time.Now())
	key := formatBlockKey(ip, currentBucket)
	_, found := b.cache.Get(key)
	return found
}

// TODO: Add a Block(ip string) method here?
// func (b *BlockIp) Block(ip string) error { ... }
// This would require access to logger potentially.

// getTimeBucket calculates the time bucket based on the configured duration.
func getTimeBucket(t time.Time) int64 {
	return t.Unix() / bucketDurationSec
}

// formatBlockKey creates a unique cache key for an IP address and time bucket.
func formatBlockKey(ip string, bucket int64) string {
	return fmt.Sprintf("block|%s|%d", ip, bucket)
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
