package config

// Limiter holds the rate limiter configuration. The section splits into the
// two standard algorithms, each with its own table: token_bucket and
// fixed_window. Every entry is keyed by a label you choose, for example
// "upload", and code looks it up with App.Limiter(label); callers that use
// the same label share it. The framework seeds its own entries; other
// entries are created via ripc scaffold.
type Limiter struct {
	TokenBucket TokenBucket `toml:"token_bucket"`
	FixedWindow FixedWindow `toml:"fixed_window"`
}

// TokenBucket lists the configured token-bucket limiters, keyed by a label
// you choose.
type TokenBucket map[string]TokenBucketEntry

// TokenBucketEntry describes one named rate limiter.
//
// Burst is how many calls the limiter lets through without waiting. Once they
// are gone it lets one more through every Limit, and the number builds back up
// while it is idle, so the full Burst returns.
//
// Limit is the wait between calls once the burst is gone. With Burst 5 and
// Limit '2s', five calls go through with no wait, then one every two seconds.
type TokenBucketEntry struct {
	Limit Duration `toml:"limit" comment:"Wait between calls once the burst is gone, e.g. '2s'"`
	Burst int      `toml:"burst" comment:"Calls allowed without waiting, e.g. 5"`
}

// FixedWindow lists the configured fixed-window limiters, keyed by a label
// you choose.
type FixedWindow map[string]FixedWindowEntry

// FixedWindowEntry describes one named keyed fixed-window limiter. The state
// is per identity: each identity gets Max calls per Window. Prefix is required;
// it namespaces the cache key.
type FixedWindowEntry struct {
	Prefix string   `toml:"prefix" comment:"Cache key prefix, e.g. 'pr_'"`
	Window Duration `toml:"window" comment:"Bucket width, e.g. '1h'"`
	Max    int64    `toml:"max" comment:"Calls allowed per window per identity, e.g. 1"`
}
