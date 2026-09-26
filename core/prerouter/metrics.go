package prerouter

import (
	"net/http"
	"strconv"

	"github.com/caasmo/restinpieces/core"
)

// Metrics is a Go middleware for collecting HTTP request metrics.
type Metrics struct {
	app *core.App
}

// NewMetrics creates a new Metrics middleware.
func NewMetrics(app *core.App) *Metrics {
	return &Metrics{
		app: app,
	}
}

// Execute is the middleware handler function that wraps the next http.Handler
// to collect metrics.
func (m *Metrics) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip metrics collection if not activated
		if !m.app.Config().Metrics.Activated {
			next.ServeHTTP(w, r)
			return
		}

		metric := m.app.Metric()
		if metric == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Check if we already have a ResponseRecorder from earlier middleware
		rec, ok := w.(*core.ResponseRecorder)
		if !ok {
			// Log error but continue processing
			m.app.Logger().Error("metrics middleware: expected core.ResponseRecorder but got different type",
				"type", "ResponseRecorder",
				"got", w,
			)
			next.ServeHTTP(w, r)
			return
		}

		// Delegate to the next handler in the chain.
		next.ServeHTTP(rec, r)

		status := strconv.Itoa(rec.Status)
		metric.RequestsTotal.WithLabelValues(status).Inc()
	})
}
