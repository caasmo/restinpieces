package prerouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

// BlockEndpointsMismatch blocks SDK requests with a stale endpoints hash.
type BlockEndpointsMismatch struct {
	app *core.App
}

// NewBlockEndpointsMismatch creates a new endpoints hash mismatch middleware instance.
func NewBlockEndpointsMismatch(app *core.App) *BlockEndpointsMismatch {
	return &BlockEndpointsMismatch{
		app: app,
	}
}

// Execute wraps the next handler with endpoints hash mismatch detection.
// Rules:
//   - If not activated, pass through.
//   - If the request has no hash header, pass through (bootstrap case).
//   - If the request targets the list-endpoints path, pass through (recovery path).
//   - If the hash matches, pass through.
//   - If the hash differs, return an error response.
func (b *BlockEndpointsMismatch) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := b.app.Config()

		if !cfg.EndpointsBlockMismatch.Activated {
			next.ServeHTTP(w, r)
			return
		}

		clientHash := r.Header.Get(core.HeaderEndpointsHash)

        // this middleware is for sdk only
		// No header: bootstrap case, let through
		if clientHash == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Exempt the list-endpoints path (recovery path)
		listEndpointsPath := cfg.Endpoints.Path(cfg.Endpoints.ListEndpoints)
		if r.URL.Path == listEndpointsPath {
			next.ServeHTTP(w, r)
			return
		}

		serverHash := cfg.Endpoints.Hash()
		if clientHash != serverHash {
			core.WriteJsonError(w, core.ErrorEndpointsHashMismatch)
			return
		}

		next.ServeHTTP(w, r)
	})
}
