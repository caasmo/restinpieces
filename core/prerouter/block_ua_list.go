package prerouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

// BlockUaList handles blocking requests based on User-Agent header matching a regex.
type BlockUaList struct {
	app *core.App // Use App to access config
}

// NewBlockUaList creates a new User-Agent block list middleware instance.
func NewBlockUaList(app *core.App) *BlockUaList {
	return &BlockUaList{
		app: app,
	}
}

func (b *BlockUaList) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := b.app.Config()
		blockUaCfg := cfg.BlockUa

		// Check if User-Agent blocking is activated and has a valid regex
		if !blockUaCfg.Activated || blockUaCfg.List.Regexp == nil {
			next.ServeHTTP(w, r)
			return
		}

		userAgent := r.UserAgent()
		if blockUaListCfg.List.MatchString(userAgent) {
			// 403 Forbidden is more appropriate for actively blocking a client
			w.WriteHeader(http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
