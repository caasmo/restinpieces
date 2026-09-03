package config

import "sync/atomic"

// Provider holds the application configuration and allows for atomic updates.
type Provider struct {
	pointer atomic.Pointer[Config] // Holds the current *Config
}

// NewProvider creates a new configuration provider with the initial config.
// It panics if the initialConfig is nil.
func NewProvider(c *Config) *Provider {
	if c == nil {
		panic("initial config cannot be nil")
	}
	p := &Provider{}
	p.pointer.Store(c)
	return p
}

// Get returns the current configuration snapshot.
// It's safe for concurrent use.
func (p *Provider) Get() *Config {
	return p.pointer.Load()
}

// Update atomically swaps the current configuration with the new one.
// The caller is responsible for ensuring newConfig is not nil.
func (p *Provider) Update(newConfig *Config) {
	// Assume newConfig is valid as the check is moved to the caller (signal handler)
	p.pointer.Store(newConfig)
	// Logging is now handled by the caller (e.g., signal handler in main.go)
}

// Pointer returns the current-config box: the atomic pointer the
// provider publishes into. Consumers that must read the current
// configuration over time (e.g. daemons) hold this box and call
// Load() at each decision point; Get and Update are thin wrappers
// over the same box.
func (p *Provider) Pointer() *atomic.Pointer[Config] {
	return &p.pointer
}
