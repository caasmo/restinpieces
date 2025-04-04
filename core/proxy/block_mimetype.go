package proxy

import "github.com/caasmo/restinpieces/core"

// BlockMimetype handles blocking requests based on MIME types.
// It uses a whitelist defined in the configuration.
type BlockMimetype struct {
	// Reference to the core application for accessing logger, config, etc.
	app *core.App
	// TODO: Add fields for storing the whitelist efficiently (e.g., a map[string]struct{})
}

// NewBlockMimetype creates a new instance of BlockMimetype.
func NewBlockMimetype(app *core.App) *BlockMimetype {
	// TODO: Initialize the internal whitelist map from app.Config().Proxy.MimetypesWhitelist
	return &BlockMimetype{
		app: app,
	}
}

// IsEnabled checks if the mimetype blocking feature is enabled.
// Placeholder: Assumes enabled if this blocker is used. Config check needed.
func (b *BlockMimetype) IsEnabled() bool {
	// TODO: Check app.Config().Proxy.MimetypesWhitelist is not empty or a specific enabled flag?
	return true // Placeholder
}

// IsBlocked checks if a given mimetype is blocked (i.e., not in the whitelist).
// Placeholder: Always returns false.
func (b *BlockMimetype) IsBlocked(mimetype string) bool {
	// TODO: Implement check against the internal whitelist map
	return false // Placeholder
}

// Block is a placeholder for the Blocker interface. Mimetype blocking is typically about
// allowing/denying based on type, not actively blocking an entity like an IP.
// This might log a warning or potentially add the mimetype to a temporary blocklist if needed.
func (b *BlockMimetype) Block(mimetype string) error {
	b.app.Logger().Warn("Block called on BlockMimetype (placeholder)", "mimetype", mimetype)
	return nil // Placeholder
}

// Process is a placeholder for the Blocker interface. Mimetype blocking doesn't
// typically involve processing like a sketch.
func (b *BlockMimetype) Process(mimetype string) []string {
	// Nothing to process like an IP sketch
	return nil // Placeholder
}

// TODO: Add methods like IsAllowed(mimetype string) bool
// IsAllowed would be the inverse of IsBlocked.
// func (b *BlockMimetype) IsAllowed(mimetype string) bool {
// 	 return !b.IsBlocked(mimetype)
// }
