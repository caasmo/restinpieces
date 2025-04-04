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

// IsBlocked checks if a given mimetype (from Content-Type header) is blocked (i.e., not in the whitelist).
func (b *BlockMimetype) IsBlocked(contentTypeHeader string) bool {
	if contentTypeHeader == "" {
		// Decide policy for missing Content-Type. Usually allow unless specifically configured.
		// For now, assume allowed if header is missing.
		return false
	}

	// Parse the media type and parameters, handle potential errors
	mediaType, _, err := mime.ParseMediaType(contentTypeHeader)
	if err != nil {
		b.app.Logger().Warn("Failed to parse Content-Type header", "header", contentTypeHeader, "error", err)
		// Decide policy for unparseable Content-Type. Block for safety?
		return true // Block if unparseable
	}

	// Check if the parsed media type (lowercase) exists in the whitelist map
	_, found := b.whitelist[strings.ToLower(mediaType)]
	return !found // Blocked if NOT found in whitelist
}

// Block logs that a block attempt was triggered for a mimetype.
// The actual HTTP 415 response is sent by the Proxy's ServeHTTP.
func (b *BlockMimetype) Block(mimetype string) error {
	// Log the effective block action (which is returning 415 in ServeHTTP)
	b.app.Logger().Warn("Blocked request due to unsupported Content-Type", "mimetype", mimetype)
	// No actual state change needed here like in IP blocking
	return nil
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
