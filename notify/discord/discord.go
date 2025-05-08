package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/h<ctrl61>ttp"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/caasmo/restinpieces/notify"
)

// Options configures the Notifier.
type Options struct {
	WebhookURL   string
	APIRateLimit rate.Limit
	APIBurst     int
	SendTimeout  time.Duration
}

type payload struct {
	Content string `json:"content"`
}

const (
	// discordMaxMessageLength is the maximum character limit for a Discord message.
	// Messages longer than this will be truncated.
	discordMaxMessageLength = 2000
)

// Notifier implements the notify.Notifier interface for sending notifications to Discord.
// It is safe for concurrent use as its fields are either immutable after creation or are
// concurrency-safe types (like *slog.Logger, *http.Client, *rate.Limiter).
// The Send method is non-blocking and launches a goroutine for actual HTTP dispatch.
type Notifier struct {
	opts           Options
	logger         *slog.Logger
	httpClient     *http.Client
	apiRateLimiter *rate.Limiter
}

// New creates a new Notifier.
func New(opts Options, logger *slog.Logger) (*Notifier, error) {
	if opts.WebhookURL == "" {
		return nil, fmt.Errorf("discord: WebhookURL is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("discord: logger is required")
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
		logger:         logger,
		apiRateLimiter: rate.NewLimiter(opts.APIRateLimit, opts.APIBurst),
		httpClient:     &http.Client{
			// Timeout on httpClient is for the entire attempt including connection, redirects, reading body.
			// We'll use a separate context with timeout for the request in the goroutine.
		},
	}, nil
}

func (dn *Notifier) formatMessage(n notify.Notification) string {
	mainMessage := fmt.Sprintf("[%s] from *%s*:\n> %s\n",
		n.Type.String(),
		n.Source,
		n.Message)

	var fieldsFormatted []string
	if len(n.Fields) > 0 {
		for k, v := range n.Fields {
			if v == nil { // Skip fields with nil values
				continue
			}
			valStr := fmt.Sprintf("%v", v)
			// Add field if key and its string representation of value are non-empty
			if k != "" && valStr != "" {
				// Each field line includes its own newline
				fieldsFormatted = append(fieldsFormatted, fmt.Sprintf("> %s: `%s`\n", k, valStr))
			}
		}
	}

	var fieldsSection string
	if len(fieldsFormatted) > 0 {
		// Join with an empty separator as each part in fieldsFormatted already ends with \n
		fieldsSection = "\n**Fields**:\n" + strings.Join(fieldsFormatted, "")
	}

	content := mainMessage + fieldsSection
	if len(content) > discordMaxMessageLength {
		// Truncate and add ellipsis, ensuring space for "..."
		return content[:discordMaxMessageLength-3] + "..."
	}
	return content
}

// Send implements the notifier.Notifier interface.
// It is non-blocking. It attempts to acquire a rate limit token and, if successful,
// launches a goroutine to send the notification to Discord.
// Errors returned by Send are for immediate processing issues (e.g., invalid type, level too low).
// Errors during the actual HTTP send are logged via the appLogger.
func (dn *Notifier) Send(_ context.Context, n notify.Notification) error {
	// Removed the check that restricted sending to only notify.Alarm type.
	// Now all notification types will be processed.

	if !dn.apiRateLimiter.Allow() {
		dn.logger.Warn("discord: API rate limit reached or burst active, dropping notification",
			"source", n.Source, "message", n.Message)
		return nil // Indicate successful processing (by dropping it as per rate limit policy)
	}

	// Launch a goroutine to handle the actual sending.
	go func(notif notify.Notification) {
		// Create a new context with timeout for this specific send operation.
		// The original context from Send() is not used in the goroutine to avoid cancellation
		// if the calling request finishes before the notification is sent.
		sendCtx, cancel := context.WithTimeout(context.Background(), dn.opts.SendTimeout)
		defer cancel()

		formattedMessage := dn.formatMessage(notif)
		payload := payload{Content: formattedMessage}
		jsonBody, err := json.Marshal(payload)
		if err != nil {
			dn.logger.Error("discord: goroutine failed to marshal payload",
				"source", notif.Source, "message", notif.Message, "error", err)
			return
		}

		req, err := http.NewRequestWithContext(sendCtx, http.MethodPost, dn.opts.WebhookURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			dn.logger.Error("discord: goroutine failed to create request",
				"source", notif.Source, "message", notif.Message, "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := dn.httpClient.Do(req)
		if err != nil {
			dn.logger.Error("discord: goroutine failed to send to discord",
				"source", notif.Source, "message", notif.Message, "error", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			dn.logger.Error("discord: goroutine received non-2xx status from Discord",
				"status_code", resp.StatusCode, "source", notif.Source, "message", notif.Message)
			if resp.StatusCode == http.StatusTooManyRequests {
				dn.logger.Warn("discord: goroutine Received 429 Too Many Requests. Rate limit settings may need adjustment.")
			}
			// Potentially read and log resp.Body here for more details
			return
		}

		dn.logger.Log(sendCtx, slog.LevelDebug, "Successfully sent alarm notification to Discord via goroutine",
			"source", notif.Source, "message", notif.Message)

	}(n) // Pass 'n' by value to the goroutine to avoid data races if 'n' was a pointer to shared mutable data.

	return nil // Notification successfully enqueued/processed for sending.
}
