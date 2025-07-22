package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

func TestMetricsHandler(t *testing.T) {
	testCases := []struct {
		name           string
		config         config.Metrics
		remoteAddr     string
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "metrics disabled",
			config: config.Metrics{
				Enabled: false,
			},
			remoteAddr:     "127.0.0.1:12345",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "not found",
		},
		{
			name: "ip not allowed",
			config: config.Metrics{
				Enabled:    true,
				AllowedIPs: []string{"192.168.1.1"},
			},
			remoteAddr:     "127.0.0.1:12345",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "not found",
		},
		{
			name: "ip allowed",
			config: config.Metrics{
				Enabled:    true,
				AllowedIPs: []string{"127.0.0.1"},
			},
			remoteAddr:     "127.0.0.1:12345",
			expectedStatus: http.StatusOK,
			expectedBody:   "go_goroutines",
		},
		{
			name: "ip in list of allowed ips",
			config: config.Metrics{
				Enabled:    true,
				AllowedIPs: []string{"192.168.1.1", "127.0.0.1"},
			},
			remoteAddr:     "127.0.0.1:12345",
			expectedStatus: http.StatusOK,
			expectedBody:   "go_goroutines",
		},
		{
			name: "empty remote addr",
			config: config.Metrics{
				Enabled:    true,
				AllowedIPs: []string{"127.0.0.1"},
			},
			remoteAddr:     ":",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "err_invalid_input",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			provider := &config.Provider{}
			provider.Update(&config.Config{Metrics: tc.config})

			app := &App{}
			app.SetConfigProvider(provider)
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			req.RemoteAddr = tc.remoteAddr
			rr := httptest.NewRecorder()

			// Execute
			app.MetricsHandler(rr, req)

			// Verify status code
			if rr.Code != tc.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					rr.Code, tc.expectedStatus)
			}

			// Verify body
			body := rr.Body.String()
			if !strings.Contains(body, tc.expectedBody) {
				t.Errorf("handler returned unexpected body: got %q want to contain %q",
					body, tc.expectedBody)
			}

			// For successful requests, also check content type
			if tc.expectedStatus == http.StatusOK {
				contentType := rr.Header().Get("Content-Type")
				expectedContentTypePrefix := "text/plain; version=0.0.4"
				if !strings.HasPrefix(contentType, expectedContentTypePrefix) {
					t.Errorf("handler returned wrong content type: got %q want prefix %q",
						contentType, expectedContentTypePrefix)
				}
			}
		})
	}
}
