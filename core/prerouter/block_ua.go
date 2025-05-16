package prerouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

// BlockUa handles blocking requests based on User-Agent header matching a regex.
type BlockUa struct {
	app *core.App // Use App to access config
}

// NewBlockUa creates a new User-Agent blocking middleware instance.
func NewBlockUa(app *core.App) *BlockUa {
	return &BlockUa{
		app: app,
	}
}

func (b *BlockUa) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := b.app.Config()
		blockUaCfg := cfg.BlockUa

		// Check if User-Agent blocking is activated and has a valid regex
		if !blockUaCfg.Activated || blockUaCfg.List.Regexp == nil {
			next.ServeHTTP(w, r)
			return
		}

		userAgent := r.UserAgent()
		if blockUaCfg.List.MatchString(userAgent) {
			// 403 Forbidden is more appropriate for actively blocking a client
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
