package discord

import (
	"context"
	"fmt"
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

func TestFormatMessage(t *testing.T) {
	dn := &Notifier{}

	testCases := []struct {
		name         string
		notification notify.Notification
		expected     string
	}{
		{
			name: "Simple alarm",
			notification: notify.Notification{
				Type:	notify.Alarm,
				Source:	"test-source",
				Message: "this is a test",
			},
			expected: fmt.Sprintf(discordMessageFormat, notify.Alarm.String(), "test-source", "this is a test"),
		},
		{
			name: "Metric with fields",
			notification: notify.Notification{
				Type:	notify.Metric,
				Source:	"metric-source",
				Message: "metric update",
				Fields: map[string]interface{}{
					"field1": "value1",
					"field2": 123,
				},
			},
			expected: fmt.Sprintf(discordMessageFormat, notify.Metric.String(), "metric-source", "metric update") +
				"\n**Fields**:\n> field1: `value1`\n> field2: `123`\n",
		},
		{
			name: "Message with nil and empty fields",
			notification: notify.Notification{
				Type:	notify.Alarm,
				Source:	"source",
				Message: "message",
				Fields: map[string]interface{}{
					"real_field": "real_value",
					"nil_field":	nil,
					"empty_val":	"",
					"":			"empty_key",
				},
			},
			expected: fmt.Sprintf(discordMessageFormat, notify.Alarm.String(), "source", "message") +
				"\n**Fields**:\n> real_field: `real_value`\n",
		},
		{
			name: "Message exceeding max length",
			notification: notify.Notification{
				Type:	notify.Alarm,
				Source:	"long-source",
				Message: strings.Repeat("a", 2001),
			},
			expected: fmt.Sprintf(discordMessageFormat, notify.Alarm.String(), "long-source", strings.Repeat("a", 2001))[:discordMaxMessageLength-3] + "...",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formatted := dn.formatMessage(tc.notification)
			if formatted != tc.expected {
				t.Errorf("Expected formatted message:\n%q\nGot:\n%q", tc.expected, formatted)
			}
		})
	}
}

func TestSend(t *testing.T) {
	var handler http.Handler
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	notifier, _ := New(config.Discord{WebhookURL: server.URL}, logger)

	testCases := []struct {
		name           string
		handler        http.Handler
		notification   notify.Notification
		expectLog      bool
		expectedStatus int
	}{
		{
			name: "Successful send",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
			notification:   notify.Notification{Type: notify.Alarm, Source: "test", Message: "success"},
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "Server error",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
			notification:   notify.Notification{Type: notify.Alarm, Source: "test", Message: "fail"},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "Rate limit drop",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
			}),
			notification:   notify.Notification{Type: notify.Alarm, Source: "test", Message: "ratelimit"},
			expectedStatus: http.StatusTooManyRequests,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler = tc.handler
			err := notifier.Send(context.Background(), tc.notification)
			if err != nil {
				t.Fatalf("Send() returned an error: %v", err)
			}
			// Give the goroutine time to run
			time.Sleep(50 * time.Millisecond)
		})
	}
}
