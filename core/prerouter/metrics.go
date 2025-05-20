package prerouter

import (
	"net/http"
	"strconv"

	"github.com/caasmo/restinpieces/core"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricName          = "http_server_requests_total"
	metricHelp          = "Total number of HTTP requests handled by the server, labeled by status code."
	statusCodeLabelName = "code"
)

// Metrics is a Go middleware for collecting HTTP request metrics.
type Metrics struct {
	app           *core.App
	requestsTotal *prometheus.CounterVec
}

// NewMetrics creates a new Metrics instance.
// It registers a Prometheus counter vector for tracking requests by status code.
// This function will panic if metric registration fails (e.g., due to a name collision with an
// incompatible metric type or other registration errors).
func NewMetrics(app *core.App) *Metrics {
	labelNames := []string{statusCodeLabelName}

	counterVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: metricName,
			Help: metricHelp,
		},
		labelNames,
	)

	// Register the counter vector with default registry
	if err := prometheus.DefaultRegisterer.Register(counterVec); err != nil {
		panic("metrics: failed to register requests_total counter vec: " + err.Error())
	}

	m := &Metrics{
		app:           app,
		requestsTotal: counterVec,
	}

	app.Logger().Info("metrics middleware initialized")

	return m
}

// Execute is the middleware handler function that wraps the next http.Handler
// to collect metrics.
func (m *Metrics) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		m.requestsTotal.WithLabelValues(status).Inc()
	})
}
