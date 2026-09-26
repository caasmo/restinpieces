package prerouter

import (
	"net/http"

	"github.com/caasmo/restinpieces/core"
)

// Metrics hands every finished response to the application's
// core.MetricsRecorder.
//
// It is added right after the ResponseRecorder, so the writer it receives is
// the framework recorder. A middleware added between the two that replaces
// the writer makes the type check below fail; the middleware then logs an
// error and skips recording.
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
// and hands the finished response to the app's MetricsRecorder.
func (m *Metrics) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip metrics collection if not activated
		if !m.app.Config().Metrics.Activated {
			next.ServeHTTP(w, r)
			return
		}

		metricsRecorder := m.app.Metrics()
		if metricsRecorder == nil {
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

		metricsRecorder.Record(rec)
	})
}
