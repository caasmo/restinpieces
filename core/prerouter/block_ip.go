package prerouter

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/topk"
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

// The primary goal of this middleware is to act as a simple, robust circuit
// breaker to try to prevent server collapse, not to be a nuanced,
// application-aware rate-limiting system (quotas, etc)
type BlockIp struct {
	app    *core.App
	sketch *topk.TopKSketch
}

// sketchParams holds the configuration for a TopK sketch.
type sketchParams struct {
	k          int
	windowSize int
	width      int
	depth      int
	tickSize   uint64
}

// sketchLevels defines the parameter presets for different sensitivity levels.
// These presets balance memory usage against detection accuracy.
// - "low":    ~10 KB memory. For low-traffic sites (< 50 RPS). Less accurate.
// - "medium": ~120 KB memory. Balanced profile for most use cases (50-500 RPS).
// - "high":   ~640 KB memory. For high-traffic sites (> 500 RPS) needing max accuracy.
var sketchLevels = map[string]sketchParams{
	"low": {
		k:          2,
		windowSize: 5,
		width:      256,
		depth:      2,
		tickSize:   100,
	},
	"medium": {
		k:          3,
		windowSize: 10,
		width:      1024,
		depth:      3,
		tickSize:   100,
	},
	"high": {
		k:          5,
		windowSize: 10,
		width:      4096,
		depth:      4,
		tickSize:   200,
	},
}

// NewBlockIp creates a new BlockIp instance with the given cache and logger.
func NewBlockIp(app *core.App) *BlockIp {
	level := app.Config().BlockIp.Level
	// The level is validated in config.Validate, so we can safely assume it exists in the map.
	params := sketchLevels[level]

	cs := topk.New(params.k, params.windowSize, params.width, params.depth, params.tickSize)

	return &BlockIp{
		app:    app,
		sketch: cs,
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
					b.app.Logger().Error("Error processing IP in blocker", "ip", ip, "error", err)
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
	_, found := b.app.Cache().Get(key)
	return found
}

// Block adds the given IP to the block list.
func (b *BlockIp) Block(ip string) error {
	now := time.Now()
	currentBucket := getTimeBucket(now)
	nextBucket := currentBucket + 1
	//until := now.Add(blockingDuration)

	// Block in current bucket with full blocking duration
	currentKey := formatBlockKey(ip, currentBucket)
	// Use the internal cache instance (b.cache) and logger
	if !b.app.Cache().SetWithTTL(currentKey, true, defaultBlockCost, blockingDuration) {
		b.app.Logger().Error("failed to block IP in current bucket", "ip", ip, "bucket", currentBucket)
		return fmt.Errorf("failed to block IP %s in current bucket %d", ip, currentBucket)
	}
	b.app.Logger().Info("IP blocked in current bucket",
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
		if !b.app.Cache().SetWithTTL(nextKey, true, defaultBlockCost, ttlNext) {
			b.app.Logger().Error("failed to block IP in next bucket", "ip", ip, "bucket", nextBucket)
			return fmt.Errorf("failed to block IP %s in next bucket %d", ip, nextBucket)
		}
		b.app.Logger().Info("IP blocked in next bucket",
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
	cfg := b.app.Config().BlockIp
	blockedIPs := b.sketch.ProcessTick(ip, cfg.Level, cfg.ActivationRPS)

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
		b.app.Logger().Info("IPs to be blocked", "ips", blockedIPs)
		go func(ips []string) {
			for _, ip := range ips {
				if err := b.Block(ip); err != nil {
					b.app.Logger().Error("failed to block IP", "ip", ip, "error", err)
				}
			}
		}(blockedIPs)
	}

	// Return nil as errors are handled within the goroutine or sketch processing
	return nil
}
