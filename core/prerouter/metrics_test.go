package prerouter

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newTestMetricsMiddleware creates a Metrics middleware instance for testing.
// It manually constructs the struct to avoid using the global Prometheus registry,
// allowing for isolated test runs.
func newTestMetricsMiddleware(app *core.App) (*Metrics, *prometheus.CounterVec) {
	counterVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_server_requests_total_test", // Use a unique name for testing
		},
		[]string{"code"},
	)

	// Because this test file is in the same package (prerouter), we can
	// access the unexported fields of the Metrics struct to build it manually.
	metricsMiddleware := &Metrics{
		app:           app,
		requestsTotal: counterVec,
	}

	return metricsMiddleware, counterVec
}

func TestMetricsMiddleware(t *testing.T) {
	testCases := []struct {
		name               string
		metricsActive      bool
		responseStatusCode int
		requestCount       int
		useResponseRecorder bool // To test the robustness case
		expectedMetricValue float64
	}{
		{
			name:                "Case: Metrics Activated - Successful Request (200 OK)",
			metricsActive:       true,
			responseStatusCode:  http.StatusOK,
			requestCount:        1,
			useResponseRecorder: true,
			expectedMetricValue: 1,
		},
		{
			name:                "Case: Metrics Activated - Client Error (404 Not Found)",
			metricsActive:       true,
			responseStatusCode:  http.StatusNotFound,
			requestCount:        1,
			useResponseRecorder: true,
			expectedMetricValue: 1,
		},
		{
			name:                "Case: Metrics Activated - Server Error (500 Internal Server Error)",
			metricsActive:       true,
			responseStatusCode:  http.StatusInternalServerError,
			requestCount:        1,
			useResponseRecorder: true,
			expectedMetricValue: 1,
		},
		{
			name:                "Case: Metrics Deactivated",
			metricsActive:       false,
			responseStatusCode:  http.StatusOK,
			requestCount:        1,
			useResponseRecorder: true,
			expectedMetricValue: 0,
		},
		{
			name:                "Case: Multiple Requests with the Same Status",
			metricsActive:       true,
			responseStatusCode:  http.StatusOK,
			requestCount:        3,
			useResponseRecorder: true,
			expectedMetricValue: 3,
		},
		{
			name:                "Case: Robustness - Missing core.ResponseRecorder",
			metricsActive:       true,
			responseStatusCode:  http.StatusOK,
			requestCount:        1,
			useResponseRecorder: false, // This is the key for this test case
			expectedMetricValue: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup: Create a mock app and set the configuration.
			mockApp := &core.App{}
			// We need a logger for the robustness case where an error is logged.
			// We use a discard handler as we don't need to check the output.
			discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
			mockApp.SetLogger(discardLogger)
			cfg := &config.Config{
				Metrics: config.Metrics{
					Activated: tc.metricsActive,
				},
			}
			provider := config.NewProvider(cfg)
			mockApp.SetConfigProvider(provider)

			// Setup: Create the test middleware and its associated counter.
			metricsMiddleware, counter := newTestMetricsMiddleware(mockApp)

			// Setup: Create the final handler that sets the desired status code.
			finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.responseStatusCode)
			})

			// Setup: Create the full middleware chain.
			var handler http.Handler = finalHandler
			// The Metrics middleware depends on the Recorder middleware.
			handler = metricsMiddleware.Execute(handler)
			if tc.useResponseRecorder {
				recorderMiddleware := NewRecorder(mockApp)
				handler = recorderMiddleware.Execute(handler)
			}

			// Execution: Run the request(s) through the chain.
			for i := 0; i < tc.requestCount; i++ {
				req := httptest.NewRequest("GET", "/", nil)
				// The handler chain is already configured for the specific test case.
				// We just need to provide a ResponseWriter.
				var rw http.ResponseWriter = httptest.NewRecorder()
				handler.ServeHTTP(rw, req)
			}

			// Verification: Check the metric value.
			statusCodeStr := strconv.Itoa(tc.responseStatusCode)
			metricValue := testutil.ToFloat64(counter.WithLabelValues(statusCodeStr))

			if metricValue != tc.expectedMetricValue {
				t.Errorf("Expected metric value for code %s to be %.1f, but got %.1f",
					statusCodeStr, tc.expectedMetricValue, metricValue)
			}
		})
	}
}


