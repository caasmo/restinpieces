package prerouter

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

func TestRecoveryMiddleware(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	testCases := []struct {
		name               string
		next               http.Handler
		expectedStatusCode int
		expectLog          bool
	}{
		{
			name:               "Case: Handler Panics",
			next:               panicHandler,
			expectedStatusCode: http.StatusInternalServerError,
			expectLog:          true,
		},
		{
			name:               "Case: Handler Succeeds",
			next:               &mockNextHandler{},
			expectedStatusCode: http.StatusOK,
			expectLog:          false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logBuffer := new(bytes.Buffer)
			memHandler := newMemoryHandler(logBuffer)
			mockApp := &core.App{}
			mockApp.SetLogger(slog.New(memHandler))
			mockApp.SetConfigProvider(config.NewProvider(config.NewDefaultConfig()))

			req := httptest.NewRequest("GET", "/boom", nil)
			rr := httptest.NewRecorder()

			handler := NewRecovery(mockApp).Execute(tc.next)
			handler.ServeHTTP(rr, req)

			if rr.Code != tc.expectedStatusCode {
				t.Errorf("Expected status code %d, but got %d", tc.expectedStatusCode, rr.Code)
			}

			if !tc.expectLog {
				if logBuffer.Len() != 0 {
					t.Errorf("Expected no log entry, but got %q", logBuffer.String())
				}
				return
			}

			logRecord, err := memHandler.LastRecord()
			if err != nil {
				t.Fatalf("Failed to parse log output: %v", err)
			}

			if msg, _ := logRecord["msg"].(string); msg != "recovered from panic" {
				t.Errorf("Expected log message 'recovered from panic', got '%v'", msg)
			}
			if level, _ := logRecord["level"].(string); level != "ERROR" {
				t.Errorf("Expected log level ERROR, got '%v'", level)
			}
			if stack, _ := logRecord["stack"].(string); stack == "" {
				t.Error("Expected a stack trace in the log entry")
			}
		})
	}
}

func TestRecoveryMiddleware_AbortHandler(t *testing.T) {
	logBuffer := new(bytes.Buffer)
	memHandler := newMemoryHandler(logBuffer)
	mockApp := &core.App{}
	mockApp.SetLogger(slog.New(memHandler))
	mockApp.SetConfigProvider(config.NewProvider(config.NewDefaultConfig()))

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler := NewRecovery(mockApp).Execute(panicHandler)

	defer func() {
		panicValue := recover()
		if panicValue != http.ErrAbortHandler {
			t.Errorf("Expected ErrAbortHandler to be re-panicked, but got %v", panicValue)
		}
		if logBuffer.Len() != 0 {
			t.Errorf("Expected ErrAbortHandler not to be logged, but got %q", logBuffer.String())
		}
	}()

	handler.ServeHTTP(rr, req)
}
