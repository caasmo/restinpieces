package prerouter

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/caasmo/restinpieces/core"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricsMiddlewareOpts holds configuration options for the MetricsMiddleware.
type MetricsMiddlewareOpts struct {
	// MetricName is the name of the Prometheus counter.
	// Default: "http_server_requests_total"
	MetricName string

	// MetricHelp is the help string for the Prometheus counter.
	// Default: "Total number of HTTP requests handled by the server, labeled by status code."
	MetricHelp string

	// StatusCodeLabelName is the name of the label used for the HTTP status code.
	// Default: "code"
	StatusCodeLabelName string

	// ConstLabels are static labels to be added to every metric.
	// Keys are label names, values are label values.
	ConstLabels map[string]string

	// Registry is the Prometheus registry to register the metric with.
	// If nil, prometheus.DefaultRegisterer is used.
	Registry prometheus.Registerer
}

const (
	defaultMetricName          = "http_server_requests_total"
	defaultMetricHelp          = "Total number of HTTP requests handled by the server, labeled by status code."
	defaultStatusCodeLabelName = "code"
)

// MetricsMiddleware is a Go middleware for collecting HTTP request metrics.
type MetricsMiddleware struct {
	requestsTotal    *prometheus.CounterVec
	constLabelValues []string // Pre-ordered values for const labels, to be used with status code.
}

// responseRecorder is no longer needed as we use core.ResponseRecorder

// NewMetricsMiddleware creates a new MetricsMiddleware.
// It registers a Prometheus counter vector for tracking requests by status code and any constant labels.
// This function will panic if metric registration fails (e.g., due to a name collision with an
// incompatible metric type or other registration errors). The caller is responsible for ensuring
// that metric names are unique or that registration is managed appropriately in their application.
func NewMetricsMiddleware(opts MetricsMiddlewareOpts) *MetricsMiddleware {
	metricName := opts.MetricName
	if metricName == "" {
		metricName = defaultMetricName
	}

	metricHelp := opts.MetricHelp
	if metricHelp == "" {
		metricHelp = defaultMetricHelp
	}

	statusCodeLabelName := opts.StatusCodeLabelName
	if statusCodeLabelName == "" {
		statusCodeLabelName = defaultStatusCodeLabelName
	}

	// Prepare label names for the CounterVec: status code label + sorted const label names.
	// And pre-compile the values for const labels in the correct order.
	var labelNames []string
	var constLabelValues []string

	// Status code label is always the first one.
	labelNames = append(labelNames, statusCodeLabelName)

	if len(opts.ConstLabels) > 0 {
		// Extract keys from ConstLabels and sort them to ensure consistent label ordering.
		sortedConstLabelKeys := make([]string, 0, len(opts.ConstLabels))
		for k := range opts.ConstLabels {
			sortedConstLabelKeys = append(sortedConstLabelKeys, k)
		}
		sort.Strings(sortedConstLabelKeys) // Sort keys for deterministic label order.

		constLabelValues = make([]string, 0, len(sortedConstLabelKeys))
		for _, k := range sortedConstLabelKeys {
			labelNames = append(labelNames, k) // Add sorted const label key to Prometheus label names
			constLabelValues = append(constLabelValues, opts.ConstLabels[k])
		}
	}

	counterVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: metricName,
			Help: metricHelp,
		},
		labelNames,
	)

	registry := opts.Registry
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	// Register the counter vector.
	// Panics on registration error (e.g. duplicate registration of a different metric type).
	// The user should ensure clean registration or use a custom registry with specific policies.
	if err := registry.Register(counterVec); err != nil {
		// If prometheus.AlreadyRegisteredError, it means a collector with this name exists.
		// If it's the *exact same* metric, Register is idempotent.
		// If it's a different metric type or different labels but same name, it's an error.
		// For simplicity and to adhere to "no extra features", we let this panic.
		// A more sophisticated setup might try to retrieve and use `are.ExistingCollector`
		// if `err` is `prometheus.AlreadyRegisteredError` and the existing collector is compatible.
		panic("metrics: failed to register requests_total counter vec: " + err.Error())
	}

	return &MetricsMiddleware{
		requestsTotal:    counterVec,
		constLabelValues: constLabelValues,
	}
}

// Execute is the middleware handler function that wraps the next http.Handler
// to collect metrics.
func (m *MetricsMiddleware) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if we already have a ResponseRecorder from earlier middleware
		rec, ok := w.(*core.ResponseRecorder)
		if !ok {
			// Log error but continue processing
			slog.Error("metrics middleware: expected core.ResponseRecorder but got different type",
				"type", "ResponseRecorder",
				"got", w,
			)
			next.ServeHTTP(w, r)
			return
		}

		// Delegate to the next handler in the chain.
		next.ServeHTTP(rec, r)

		// Prepare all label values for the metric.
		// The first value is always the status code.
		// The subsequent values are the pre-ordered constant label values.
		labelValues := make([]string, 1+len(m.constLabelValues))
		labelValues[0] = strconv.Itoa(rec.Status)
		copy(labelValues[1:], m.constLabelValues)

		m.requestsTotal.WithLabelValues(labelValues...).Inc()
	})
}
