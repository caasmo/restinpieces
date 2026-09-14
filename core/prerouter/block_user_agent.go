package prerouter

import (
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/core"
)

// BlockUserAgent handles blocking requests based on User-Agent header substring matching.
type BlockUserAgent struct {
	app *core.App
}

// NewBlockUserAgent creates a new User-Agent blocking middleware instance.
func NewBlockUserAgent(app *core.App) *BlockUserAgent {
	return &BlockUserAgent{
		app: app,
	}
}

func (b *BlockUserAgent) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := b.app.Config()
		blockUserAgentCfg := cfg.BlockUserAgent

		if !blockUserAgentCfg.Activated || len(blockUserAgentCfg.Agents) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		userAgent := r.UserAgent()
		// Plain Contains loop is ~100x faster than RE2 for literal lists.
		for _, agent := range blockUserAgentCfg.Agents {
			if strings.Contains(userAgent, agent) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
