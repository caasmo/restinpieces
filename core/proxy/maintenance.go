package proxy

import (
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

// Maintenance handles serving a maintenance page based on configuration.
type Maintenance struct {
	app *core.App // Use App to access config
}

// NewMaintenance creates a new maintenance middleware instance.
// It requires the core App instance to access configuration.
func NewMaintenance(app *core.App) *Maintenance {
	return &Maintenance{
		app: app,
	}
}

// Execute wraps the next handler with maintenance mode logic.
func (m *Maintenance) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := m.app.Config() // Get current config snapshot via App
		maintCfg := cfg.Maintenance

		if maintCfg.Activated {

			core.SetHeaders(w, core.HeadersMaintenancePage)

			w.WriteHeader(http.StatusServiceUnavailable) // 503 Service Unavailable

			return
		}

		next.ServeHTTP(w, r)
	})
}
