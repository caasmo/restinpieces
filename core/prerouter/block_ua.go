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
    // It requires the core App instance to access configuration.
    func NewBlockUa(app *core.App) *BlockUa {
    	return &BlockUa{
    		app: app,
    	}
    }

    // Execute wraps the next handler with User-Agent blocking logic.
    func (b *BlockUa) Execute(next http.Handler) http.Handler {
    	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    		cfg := b.app.Config() // Get current config snapshot via App
    		blockUaCfg := cfg.BlockUa

    		// Check if UA blocking is activated and the regex is compiled
    		if blockUaCfg.Activated && blockUaCfg.List.Regexp != nil {
    			userAgent := r.UserAgent()
    			// Check if the User-Agent matches the blocklist regex
    			if blockUaCfg.List.MatchString(userAgent) {
    				// Log the blocked UA? Maybe too verbose.
    				// b.app.Logger().Debug("blocking user agent", "user_agent", userAgent)

    				// Return 503 Service Unavailable, similar to Maintenance mode
    				// We don't need specific headers like Retry-After here.
    				w.WriteHeader(http.StatusServiceUnavailable)
    				return // Stop processing the request
    			}
    		}

    		// If not blocked, proceed to the next handler
    		next.ServeHTTP(w, r)
    	})
    }
