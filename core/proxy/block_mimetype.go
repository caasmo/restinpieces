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

// TODO: Add methods like IsAllowed(mimetype string) bool
