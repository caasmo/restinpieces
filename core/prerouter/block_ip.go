package prerouter

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/cache"
	"github.com/caasmo/restinpieces/topk"
	"github.com/keilerkonzept/topk/sliding"
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

// dummy TODO
func GetClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Handle error potentially, or use RemoteAddr directly if no port
		ip = r.RemoteAddr
	}

	return ip
}

// BlockIp implements the FeatureBlocker interface using a cache for storage and a TopK sketch for detection.
type BlockIp struct {
	cache  cache.Cache[string, interface{}]
	sketch *topk.TopKSketch
	logger *slog.Logger
}

// NewBlockIp creates a new BlockIp instance with the given cache and logger.
func NewBlockIp(cache cache.Cache[string, interface{}], logger *slog.Logger) *BlockIp {
	// TODO: Make sketch parameters configurable (window, segments, width, depth, tickSize)
	window := 3
	segments := 10
	width := 1024
	depth := 3
	tickSize := uint64(100) // Process sketch every 100 requests

	sketchInstance := sliding.New(window, segments, sliding.WithWidth(width), sliding.WithDepth(depth))
	logger.Info("TopK sketch memory usage", "bytes", sketchInstance.SizeBytes())

	cs := topk.NewTopkSketch(sketchInstance, tickSize)

	return &BlockIp{
		cache:  cache,
		sketch: cs,
		logger: logger,
	}
}

func (b *BlockIp) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if IP blocking is enabled first
		if b.IsEnabled() {
			// Get client IP from request using app's method
			// TODO
			ip := GetClientIP(r)

			// Check if the IP is already blocked (cache check)
			if b.IsBlocked(ip) {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			} else {
				if err := b.Process(ip); err != nil {
					b.logger.Error("Error processing IP in blocker", "ip", ip, "error", err)
				}
			}
		}

		next.ServeHTTP(w, r)
	})
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
	//until := now.Add(blockingDuration)

	// Block in current bucket with full blocking duration
	currentKey := formatBlockKey(ip, currentBucket)
	// Use the internal cache instance (b.cache) and logger
	if !b.cache.SetWithTTL(currentKey, true, defaultBlockCost, blockingDuration) {
		b.logger.Error("failed to block IP in current bucket", "ip", ip, "bucket", currentBucket)
		return fmt.Errorf("failed to block IP %s in current bucket %d", ip, currentBucket)
	}
	b.logger.Info("IP blocked in current bucket",
		"ip", ip,
		"bucket", currentBucket,
		"duration", blockingDuration)

	// Calculate time until next bucket starts
	nowUnix := now.Unix()
	timeUntilNextBucket := (nextBucket * bucketDurationSec) - nowUnix
	ttlNext := blockingDuration - time.Duration(timeUntilNextBucket)*time.Second

	if ttlNext > 0 {
		nextKey := formatBlockKey(ip, nextBucket)
		// Use the internal cache instance (b.cache) and logger
		if !b.cache.SetWithTTL(nextKey, true, defaultBlockCost, ttlNext) {
			b.logger.Error("failed to block IP in next bucket", "ip", ip, "bucket", nextBucket)
			return fmt.Errorf("failed to block IP %s in next bucket %d", ip, nextBucket)
		}
		b.logger.Info("IP blocked in next bucket",
			"ip", ip,
			"bucket", nextBucket,
			"duration", ttlNext)
	}

	return nil

}

// Process passes the IP to the underlying TopK sketch for tracking and potential blocking.
// It processes the IP using the sketch and potentially triggers blocking.
// Returns an error if the processing itself fails (unlikely here).
func (b *BlockIp) Process(ip string) error {
	blockedIPs := b.sketch.ProcessTick(ip)

	// Handle blocking asynchronously
	//
	// Even if multiple goroutines call a.BlockIP for the same IP
	// concurrently, Ristretto will handle it safely. Blocking an IP
	// multiple times is harmless if the operation is idempotent (same key).
	// Ristretto batches writes into a ring buffer, so frequent Set calls
	// for the same key will be merged efficiently. The last write (in
	// buffer order) will determine the final value.
	// Ristretto uses a buffered write mechanism (a ring buffer) to batch
	// Set/Del operations for performance.
	if len(blockedIPs) > 0 {
		b.logger.Info("IPs to be blocked", "ips", blockedIPs)
		go func(ips []string) {
			for _, ip := range ips {
				if err := b.Block(ip); err != nil {
					b.logger.Error("failed to block IP", "ip", ip, "error", err)
				}
			}
		}(blockedIPs)
	}

	// Return nil as errors are handled within the goroutine or sketch processing
	return nil
}
