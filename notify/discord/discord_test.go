package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/notify"
)

func TestNewNotifier(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	testCases := []struct {
		name        string
		config      config.Discord
		logger      *slog.Logger
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid config",
			config: config.Discord{
				WebhookURL: "http://test.com",
			},
			logger:      logger,
			expectError: false,
		},
		{
			name:        "Missing webhook URL",
			config:      config.Discord{},
			logger:      logger,
			expectError: true,
			errorMsg:    "discord: WebhookURL is required",
		},
		{
			name: "Missing logger",
			config: config.Discord{
				WebhookURL: "http://test.com",
			},
			logger:      nil,
			expectError: true,
			errorMsg:    "discord: logger is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			notifier, err := New(tc.config, tc.logger)

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected an error, but got nil")
				}
				if err.Error() != tc.errorMsg {
					t.Errorf("Expected error message %q, got %q", tc.errorMsg, err.Error())
				}
				if notifier != nil {
					t.Error("Expected notifier to be nil on error")
				}
			} else {
				if err != nil {
					t.Fatalf("Did not expect an error, but got: %v", err)
				}
				if notifier == nil {
					t.Fatal("Expected a notifier, but got nil")
				}
				if notifier.webhookURL != tc.config.WebhookURL {
					t.Errorf("Expected webhook URL %q, got %q", tc.config.WebhookURL, notifier.webhookURL)
				}
			}
		})
	}
}

func TestNotifier_Send(t *testing.T) {
	testCases := []struct {
		name             string
		notification     notify.Notification
		handlerStatus    int
		expectRequest    bool
		expectedLogParts []string
	}{
		{
			name: "Successful send with fields",
			notification: notify.Notification{
				Type:    notify.Alarm,
				Source:  "test-source",
				Message: "this is a test",
				Fields:  map[string]interface{}{"field1": "value1"},
			},
			handlerStatus: http.StatusNoContent,
			expectRequest: true,
		},
		{
			name: "Server error",
			notification: notify.Notification{
				Type:    notify.Alarm,
				Source:  "test-source",
				Message: "server error test",
			},
			handlerStatus:    http.StatusInternalServerError,
			expectRequest:    true,
			expectedLogParts: []string{"level=ERROR", "received non-2xx status"},
		},
		{
			name: "Rate limit error",
			notification: notify.Notification{
				Type:    notify.Alarm,
				Source:  "test-source",
				Message: "rate limit test",
			},
			handlerStatus:    http.StatusTooManyRequests,
			expectRequest:    true,
			expectedLogParts: []string{"level=ERROR", "level=WARN", "Too Many Requests"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuf, nil))

			requestChan := make(chan []byte, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("failed to read request body: %v", err)
				}
				w.WriteHeader(tc.handlerStatus)
				if tc.expectRequest {
					requestChan <- body
				}
			}))
			defer server.Close()

			notifier, err := New(config.Discord{WebhookURL: server.URL}, logger)
			if err != nil {
				t.Fatalf("New() failed: %v", err)
			}

			err = notifier.Send(context.Background(), tc.notification)
			if err != nil {
				t.Fatalf("Send() returned an error: %v", err)
			}

			if !tc.expectRequest {
				// If we don't expect a request, we can't wait on the channel.
				// This case would be for pre-send logic errors, which we don't have right now.
				return
			}

			select {
			case reqBody := <-requestChan:
				var payload payload
				if err := json.Unmarshal(reqBody, &payload); err != nil {
					t.Fatalf("failed to unmarshal request body: %v", err)
				}

				// Assert on the content of the message, not the exact format.
				if !strings.Contains(payload.Content, tc.notification.Source) {
					t.Errorf("expected payload to contain source %q, but it did not. Got: %q", tc.notification.Source, payload.Content)
				}
				if !strings.Contains(payload.Content, tc.notification.Message) {
					t.Errorf("expected payload to contain message %q, but it did not. Got: %q", tc.notification.Message, payload.Content)
				}
				if tc.notification.Fields != nil {
					if !strings.Contains(payload.Content, "field1") || !strings.Contains(payload.Content, "value1") {
						t.Errorf("expected payload to contain field data, but it did not. Got: %q", payload.Content)
					}
				}

			case <-time.After(100 * time.Millisecond):
				t.Fatal("timed out waiting for request")
			}

			// Give the logger a moment to catch up after the request is handled.
			time.Sleep(10 * time.Millisecond)
			logOutput := logBuf.String()

			for _, part := range tc.expectedLogParts {
				if !strings.Contains(logOutput, part) {
					t.Errorf("expected log to contain %q, but it did not. Got: %s", part, logOutput)
				}
			}
		})
	}
}
