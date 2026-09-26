package prerouter

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

// fakeMetricsRecorder counts the recorded status codes, like a real recorder
// counts responses per label.
type fakeMetricsRecorder struct {
	counts map[int]int
}

func (f *fakeMetricsRecorder) Record(rec *core.ResponseRecorder) {
	f.counts[rec.Status]++
}

// newTestMetricsMiddleware creates a Metrics middleware instance for testing.
func newTestMetricsMiddleware(app *core.App) (*Metrics, *fakeMetricsRecorder) {
	recorder := &fakeMetricsRecorder{counts: make(map[int]int)}

	app.SetMetrics(recorder)

	return NewMetrics(app), recorder
}

func TestMetricsMiddleware(t *testing.T) {
	testCases := []struct {
		name                string
		metricsActive       bool
		responseStatusCode  int
		requestCount        int
		useResponseRecorder bool // To test the robustness case
		expectedRecordCount int
	}{
		{
			name:                "Case: Metrics Activated - Successful Request (200 OK)",
			metricsActive:       true,
			responseStatusCode:  http.StatusOK,
			requestCount:        1,
			useResponseRecorder: true,
			expectedRecordCount: 1,
		},
		{
			name:                "Case: Metrics Activated - Client Error (404 Not Found)",
			metricsActive:       true,
			responseStatusCode:  http.StatusNotFound,
			requestCount:        1,
			useResponseRecorder: true,
			expectedRecordCount: 1,
		},
		{
			name:                "Case: Metrics Activated - Server Error (500 Internal Server Error)",
			metricsActive:       true,
			responseStatusCode:  http.StatusInternalServerError,
			requestCount:        1,
			useResponseRecorder: true,
			expectedRecordCount: 1,
		},
		{
			name:                "Case: Metrics Deactivated",
			metricsActive:       false,
			responseStatusCode:  http.StatusOK,
			requestCount:        1,
			useResponseRecorder: true,
			expectedRecordCount: 0,
		},
		{
			name:                "Case: Multiple Requests with the Same Status",
			metricsActive:       true,
			responseStatusCode:  http.StatusOK,
			requestCount:        3,
			useResponseRecorder: true,
			expectedRecordCount: 3,
		},
		{
			name:                "Case: Robustness - Missing core.ResponseRecorder",
			metricsActive:       true,
			responseStatusCode:  http.StatusOK,
			requestCount:        1,
			useResponseRecorder: false, // This is the key for this test case
			expectedRecordCount: 0,
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

			// Setup: Create the test middleware and its recorder.
			metricsMiddleware, recorder := newTestMetricsMiddleware(mockApp)

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

			// Verification: Check the recorded response count.
			got := recorder.counts[tc.responseStatusCode]
			if got != tc.expectedRecordCount {
				t.Errorf("recorded responses with status %d = %d, want %d",
					tc.responseStatusCode, got, tc.expectedRecordCount)
			}
		})
	}
}
