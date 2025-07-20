package prerouter

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

// memoryHandler is a custom slog.Handler that writes JSON records to an in-memory buffer
// and allows for easy inspection of the last logged record.
type memoryHandler struct {
	b *bytes.Buffer
	h slog.Handler
}

// newMemoryHandler creates a new handler that writes to the provided buffer.
func newMemoryHandler(b *bytes.Buffer) *memoryHandler {
	return &memoryHandler{
		b: b,
		h: slog.NewJSONHandler(b, nil),
	}
}

func (h *memoryHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.h.Enabled(ctx, level)
}

func (h *memoryHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.h.Handle(ctx, r)
}

func (h *memoryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &memoryHandler{b: h.b, h: h.h.WithAttrs(attrs)}
}

func (h *memoryHandler) WithGroup(name string) slog.Handler {
	return &memoryHandler{b: h.b, h: h.h.WithGroup(name)}
}

// LastRecord parses the buffer to get the last logged JSON object.
func (h *memoryHandler) LastRecord() (map[string]interface{}, error) {
	var record map[string]interface{}
	err := json.Unmarshal(h.b.Bytes(), &record)
	return record, err
}

// TestRequestLog_SuccessfulRequest verifies the happy path for a standard HTTP request.
func TestRequestLog_SuccessfulRequest(t *testing.T) {
	// --- Setup ---
	mockApp := &core.App{}
	logBuffer := new(bytes.Buffer)
	mockApp.SetLogger(slog.New(newMemoryHandler(logBuffer)))

	cfg := config.NewDefaultConfig()
	cfg.Log.Request.Activated = true
	provider := config.NewProvider(cfg)
	mockApp.SetConfigProvider(provider)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// The Recorder must wrap the logger's handler.
	handlerChain := NewRecorder(mockApp).Execute(NewRequestLog(mockApp).Execute(finalHandler))

	req := httptest.NewRequest("GET", "/test?q=1", nil)
	req.RemoteAddr = "192.0.2.1:12345"

	// --- Execution ---
	handlerChain.ServeHTTP(httptest.NewRecorder(), req)

	// --- Verification ---
	if logBuffer.Len() == 0 {
		t.Fatal("Expected a log entry, but none was written")
	}

	logRecord, err := newMemoryHandler(logBuffer).LastRecord()
	if err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if logRecord["msg"] != "http_request" {
		t.Errorf("Expected log message 'http_request', got '%v'", logRecord["msg"])
	}
	if status, _ := logRecord["status"].(float64); status != http.StatusOK {
		t.Errorf("Expected status %d, got %v", http.StatusOK, logRecord["status"])
	}
	if ip, _ := logRecord["remote_ip"].(string); ip != "192.0.2.1" {
		t.Errorf("Expected remote_ip '192.0.2.1', got '%v'", logRecord["remote_ip"])
	}
}

// TestRequestLog_Deactivated ensures no log is written when the middleware is disabled.
func TestRequestLog_Deactivated(t *testing.T) {
	// --- Setup ---
	mockApp := &core.App{}
	logBuffer := new(bytes.Buffer)
	mockApp.SetLogger(slog.New(newMemoryHandler(logBuffer)))

	cfg := config.NewDefaultConfig()
	cfg.Log.Request.Activated = false // Key for this test
	provider := config.NewProvider(cfg)
	mockApp.SetConfigProvider(provider)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handlerChain := NewRecorder(mockApp).Execute(NewRequestLog(mockApp).Execute(finalHandler))

	req := httptest.NewRequest("GET", "/", nil)

	// --- Execution ---
	handlerChain.ServeHTTP(httptest.NewRecorder(), req)

	// --- Verification ---
	if logBuffer.Len() > 0 {
		t.Errorf("Expected no log output, but got: %s", logBuffer.String())
	}
}

// TestRequestLog_FieldTruncation verifies that long fields are correctly truncated.
func TestRequestLog_FieldTruncation(t *testing.T) {
	// --- Setup ---
	mockApp := &core.App{}
	logBuffer := new(bytes.Buffer)
	mockApp.SetLogger(slog.New(newMemoryHandler(logBuffer)))

	cfg := config.NewDefaultConfig()
	cfg.Log.Request.Activated = true
	cfg.Log.Request.Limits = config.LogRequestLimits{
		URILength:       10,
		UserAgentLength: 15,
		RefererLength:   12,
		RemoteIPLength:  8,
	}
	provider := config.NewProvider(cfg)
	mockApp.SetConfigProvider(provider)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handlerChain := NewRecorder(mockApp).Execute(NewRequestLog(mockApp).Execute(finalHandler))

	longString := strings.Repeat("a", 200)
	req := httptest.NewRequest("POST", "/"+longString, nil)
	req.Header.Set("User-Agent", "user-agent-"+longString)
	req.Header.Set("Referer", "referer-"+longString)
	req.Host = longString

	// --- Execution ---
	handlerChain.ServeHTTP(httptest.NewRecorder(), req)

	// --- Verification ---
	logRecord, err := newMemoryHandler(logBuffer).LastRecord()
	if err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	expected := map[string]string{
		"uri":        "/aaaaaa...",
		"user_agent": "user-agent-a...",
		"referer":    "referer-a...",
		"host":       "aaaaa...",
	}

	for key, want := range expected {
		if got, _ := logRecord[key].(string); got != want {
			t.Errorf("Expected truncated field '%s' to be '%s', but got '%s'", key, want, got)
		}
	}
}

// TestRequestLog_HttpsRequest verifies the 'tls' field is true for HTTPS requests.
func TestRequestLog_HttpsRequest(t *testing.T) {
	// --- Setup ---
	mockApp := &core.App{}
	logBuffer := new(bytes.Buffer)
	mockApp.SetLogger(slog.New(newMemoryHandler(logBuffer)))

	cfg := config.NewDefaultConfig()
	provider := config.NewProvider(cfg)
	mockApp.SetConfigProvider(provider)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handlerChain := NewRecorder(mockApp).Execute(NewRequestLog(mockApp).Execute(finalHandler))

	req := httptest.NewRequest("GET", "/secure", nil)
	req.TLS = &tls.ConnectionState{} // Simulate HTTPS

	// --- Execution ---
	handlerChain.ServeHTTP(httptest.NewRecorder(), req)

	// --- Verification ---
	logRecord, _ := newMemoryHandler(logBuffer).LastRecord()
	if tls, ok := logRecord["tls"].(bool); !ok || !tls {
		t.Errorf("Expected 'tls' field to be true, but it was not")
	}
}

// TestRequestLog_InvalidRemoteIP verifies the fallback for a malformed RemoteAddr.
func TestRequestLog_InvalidRemoteIP(t *testing.T) {
	// --- Setup ---
	mockApp := &core.App{}
	logBuffer := new(bytes.Buffer)
	mockApp.SetLogger(slog.New(newMemoryHandler(logBuffer)))

	cfg := config.NewDefaultConfig()
	provider := config.NewProvider(cfg)
	mockApp.SetConfigProvider(provider)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handlerChain := NewRecorder(mockApp).Execute(NewRequestLog(mockApp).Execute(finalHandler))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "invalid-ip-format"

	// --- Execution ---
	handlerChain.ServeHTTP(httptest.NewRecorder(), req)

	// --- Verification ---
	logRecord, _ := newMemoryHandler(logBuffer).LastRecord()
	if ip, _ := logRecord["remote_ip"].(string); ip != "invalid-ip-format" {
		t.Errorf("Expected remote_ip to be 'invalid-ip-format', got '%v'", ip)
	}
}

// TestRequestLog_Robustness_MissingResponseRecorder ensures the middleware doesn't panic
// and logs an error if the recorder is missing.
func TestRequestLog_Robustness_MissingResponseRecorder(t *testing.T) {
	// --- Setup ---
	mockApp := &core.App{}
	errorLogBuffer := new(bytes.Buffer)
	// The middleware should log an error to the app's logger. We capture it here.
	mockApp.SetLogger(slog.New(newMemoryHandler(errorLogBuffer)))

	cfg := config.NewDefaultConfig()
	provider := config.NewProvider(cfg)
	mockApp.SetConfigProvider(provider)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This handler should still be called.
		w.Header().Set("X-Handler-Called", "true")
	})
	// The key to this test: Execute the middleware without the recorder.
	handlerChain := NewRequestLog(mockApp).Execute(finalHandler)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	// --- Execution ---
	handlerChain.ServeHTTP(rr, req)

	// --- Verification ---
	// 1. Verify an error was logged.
	if errorLogBuffer.Len() == 0 {
		t.Fatal("Expected an error to be logged, but buffer is empty")
	}
	logRecord, _ := newMemoryHandler(errorLogBuffer).LastRecord()
	if level, _ := logRecord["level"].(string); level != "ERROR" {
		t.Errorf("Expected log level to be ERROR, got %s", level)
	}
	if msg, _ := logRecord["msg"].(string); !strings.Contains(msg, "expected core.ResponseRecorder") {
		t.Errorf("Expected error message about ResponseRecorder, got %s", msg)
	}

	// 2. Verify the next handler was still called.
	if rr.Header().Get("X-Handler-Called") != "true" {
		t.Error("The next handler was not called when the recorder was missing")
	}
}