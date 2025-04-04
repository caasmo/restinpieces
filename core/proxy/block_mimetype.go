package proxy

import (
	"mime" // Import the mime package
	"strings"

	"github.com/caasmo/restinpieces/core"
)

// BlockMimetype handles blocking requests based on MIME types.
// It uses a whitelist defined in the configuration.
type BlockMimetype struct {
	// Reference to the core application for accessing logger, config, etc.
	app       *core.App
	whitelist map[string]struct{} // Use a map for efficient lookup
}

// NewBlockMimetype creates a new instance of BlockMimetype.
func NewBlockMimetype(app *core.App) *BlockMimetype {
	wl := make(map[string]struct{})
	// Initialize the whitelist map from configuration
	for _, mtype := range app.Config().Proxy.Mimetype.Whitelist {
		// Store lowercase for case-insensitive matching
		wl[strings.ToLower(mtype)] = struct{}{}
	}
	app.Logger().Info("Initialized Mimetype blocker", "whitelist_count", len(wl))
	return &BlockMimetype{
		app:       app,
		whitelist: wl,
	}
}

// IsEnabled checks if the mimetype blocking feature is enabled in the config.
func (b *BlockMimetype) IsEnabled() bool {
	// Check the Enabled flag in the configuration
	return b.app.Config().Proxy.Mimetype.Enabled
}

// IsBlocked checks if a given Content-Type header value is blocked (i.e., empty or not in the whitelist).
func (b *BlockMimetype) IsBlocked(contentTypeHeader string) bool {
	if contentTypeHeader == "" {
		// Block requests with empty Content-Type header
		b.app.Logger().Debug("Blocking request due to empty Content-Type header")
		return true
	}

	// Perform direct, case-insensitive lookup against the whitelist
	_, found := b.whitelist[strings.ToLower(contentTypeHeader)]

	if !found {
		b.app.Logger().Debug("Blocking request due to non-whitelisted Content-Type", "content_type", contentTypeHeader)
	}

	return !found // Blocked if NOT found in whitelist
}

func (b *BlockMimetype) Block(mimetype string) error {
	return nil
}

func (b *BlockMimetype) Process(mimetype string) error {
	// Nothing to process like an IP sketch
	return nil
}

