package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/caasmo/restinpieces/notify"
)

// Options configures the Notifier.
type Options struct {
	WebhookURL   string
	MinLevel     slog.Level
	APIRateLimit rate.Limit
	APIBurst     int
	SendTimeout  time.Duration
}

type payload struct {
	Content string `json:"content"`
}

// Notifier implements the notify.Notifier interface for sending notifications to Discord.
// It is safe for concurrent use as its fields are either immutable after creation or are
// concurrency-safe types (like *slog.Logger, *http.Client, *rate.Limiter).
// The Send method is non-blocking and launches a goroutine for actual HTTP dispatch.
type Notifier struct {
	opts           Options
	appLogger      *slog.Logger
	httpClient     *http.Client
	apiRateLimiter *rate.Limiter
}

// New creates a new Notifier.
func New(opts Options, appLogger *slog.Logger) (*Notifier, error) {
	if opts.WebhookURL == "" {
		return nil, fmt.Errorf("discord: WebhookURL is required")
	}
	if appLogger == nil {
		return nil, fmt.Errorf("discord: appLogger is required") // Updated error prefix
	}

	if opts.APIRateLimit == 0 {
		opts.APIRateLimit = rate.Every(2 * time.Second)
	}
	if opts.APIBurst <= 0 {
		opts.APIBurst = 5
	}
	if opts.SendTimeout <= 0 {
		opts.SendTimeout = 10 * time.Second
	}

	return &Notifier{
		opts:           opts,
		appLogger:      appLogger,
		apiRateLimiter: rate.NewLimiter(opts.APIRateLimit, opts.APIBurst),
		httpClient:     &http.Client{
			// Timeout on httpClient is for the entire attempt including connection, redirects, reading body.
			// We'll use a separate context with timeout for the request in the goroutine.
		},
	}, nil
}

func (dn *Notifier) formatMessage(n notify.Notification) string {
	var msgBuffer bytes.Buffer

	// Removed n.Level.String() from the main message
	msgBuffer.WriteString(fmt.Sprintf("[%s] from *%s*:\n> %s\n",
		n.Type.String(),
		n.Source,
		n.Message))

	// Changed from n.Tags to n.Fields
	if len(n.Fields) > 0 {
		detailsAdded := false
		// Use a temporary buffer for fields to ensure the "**Fields**" header is only added if there's content.
		tempFieldsBuffer := new(bytes.Buffer)
		for k, v := range n.Fields {
			var valStr string
			if v == nil {
				valStr = "<nil>" // Represent nil explicitly
			} else {
				valStr = fmt.Sprintf("%v", v) // Use %v for interface{}
			}

			// Add field if key and its string representation of value are non-empty
			if k != "" && valStr != "" {
				tempFieldsBuffer.WriteString(fmt.Sprintf("> %s: `%s`\n", k, valStr))
				detailsAdded = true
			}
		}

		if detailsAdded {
			msgBuffer.WriteString("\n**Fields**:\n")
			msgBuffer.Write(tempFieldsBuffer.Bytes())
		}
	}

	content := msgBuffer.String()
	if len(content) > 2000 {
		return content[:1997] + "..."
	}
	return content
}

// Send implements the notifier.Notifier interface.
// It is non-blocking. It attempts to acquire a rate limit token and, if successful,
// launches a goroutine to send the notification to Discord.
// Errors returned by Send are for immediate processing issues (e.g., invalid type, level too low).
// Errors during the actual HTTP send are logged via the appLogger.
func (dn *Notifier) Send(_ context.Context, n notify.Notification) error {
	if n.Type != notify.Alarm {
		return nil
	}

	// n.Level has been removed from notifier.Notification, so this check is removed.
	// MinLevel filtering would need to be re-evaluated if still desired,
	// possibly by adding a Level field back to Notification or handling it in the caller.

	if !dn.apiRateLimiter.Allow() {
		dn.appLogger.Warn("discord: API rate limit reached or burst active, dropping notification",
			"source", n.Source, "message", n.Message)
		return nil // Indicate successful processing (by dropping it as per rate limit policy)
	}

	// Launch a goroutine to handle the actual sending.
	go func(notificationToSend notify.Notification) {
		// Create a new context with timeout for this specific send operation.
		// The original context from Send() is not used in the goroutine to avoid cancellation
		// if the calling request finishes before the notification is sent.
		sendCtx, cancel := context.WithTimeout(context.Background(), dn.opts.SendTimeout)
		defer cancel()

		formattedMessage := dn.formatMessage(notificationToSend)
		payload := payload{Content: formattedMessage}
		jsonBody, err := json.Marshal(payload)
		if err != nil {
			dn.appLogger.Error("discord: goroutine failed to marshal payload",
				"source", notificationToSend.Source, "message", notificationToSend.Message, "error", err)
			return
		}

		req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, dn.opts.WebhookURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			dn.appLogger.Error("discord: goroutine failed to create request",
				"source", notificationToSend.Source, "message", notificationToSend.Message, "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := dn.httpClient.Do(req)
		if err != nil {
			dn.appLogger.Error("discord: goroutine failed to send to discord",
				"source", notificationToSend.Source, "message", notificationToSend.Message, "error", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			dn.appLogger.Error("discord: goroutine received non-2xx status from Discord",
				"status_code", resp.StatusCode, "source", notificationToSend.Source, "message", notificationToSend.Message)
			if resp.StatusCode == http.StatusTooManyRequests {
				dn.appLogger.Warn("discord: goroutine Received 429 Too Many Requests. Rate limit settings may need adjustment.")
			}
			// Potentially read and log resp.Body here for more details
			return
		}

		dn.appLogger.Log(sendCtx, slog.LevelDebug, "Successfully sent alarm notification to Discord via goroutine",
			"source", notificationToSend.Source, "message", notificationToSend.Message)

	}(n) // Pass 'n' by value to the goroutine to avoid data races if 'n' was a pointer to shared mutable data.

	return nil // Notification successfully enqueued/processed for sending.
}
