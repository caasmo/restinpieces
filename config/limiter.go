package config

// Limiter lists the configured rate limiters. Each map key is a name you
// choose, for example "upload", and code looks a limiter up with
// App.Limiter(name); callers that use the same name share it. The framework
// seeds its own entry, "password_hashing".
type Limiter map[string]LimiterEntry

// LimiterEntry describes one named rate limiter.
//
// Burst is how many calls the limiter lets through without waiting. Once they
// are gone it lets one more through every Limit, and the number builds back up
// while it is idle, so the full Burst returns.
//
// Limit is the wait between calls once the burst is gone. With Burst 5 and
// Limit '2s', five calls go through with no wait, then one every two seconds.
type LimiterEntry struct {
	Limit Duration `toml:"limit" comment:"Wait between calls once the burst is gone, e.g. '2s'"`
	Burst int      `toml:"burst" comment:"Calls allowed without waiting, e.g. 5"`
}
