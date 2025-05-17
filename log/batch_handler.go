package log

import (
	"context"
	"fmt"
	"github.com/caasmo/restinpieces/config"
	"log/slog"
)

// BatchHandler is a lightweight slog.Handler that sends records to a channel for batched processing.
type BatchHandler struct {
	configProvider *config.Provider   // For dynamic log levels
	recordChan     chan<- slog.Record // Write-end of the channel, provided by Daemon
	daemonCtx      context.Context    // Context from daemon for shutdown detection
    attrs          []slog.Attr 

}

// NewBatchHandler creates a new BatchHandler.
//
// configProvider: An instance of the configuration provider for dynamic log levels.
// recordChan: The write-end of a buffered channel where slog.Records will be sent.
// daemonCtx: Context from daemon to detect shutdown state.
// If any parameter is nil, this function will panic.
func NewBatchHandler(configProvider *config.Provider, recordChan chan<- slog.Record, daemonCtx context.Context) *BatchHandler {
	if configProvider == nil {
		panic("batchhandler: configProvider cannot be nil")
	}
	if recordChan == nil {
		panic("batchhandler: recordChan cannot be nil")
	}
	if daemonCtx == nil {
		panic("batchhandler: daemonCtx cannot be nil")
	}

    return &BatchHandler{
        configProvider: configProvider,
        recordChan:     recordChan,
        daemonCtx:      daemonCtx,
        attrs:          []slog.Attr{}, // Initialize empty slice
    }
}

// Enabled implements the slog.Handler interface.
// It consults the config provider to get the current logging level.
func (h *BatchHandler) Enabled(_ context.Context, level slog.Level) bool {
	conf := h.configProvider.Get()
	return level >= conf.Log.Batch.Level.Level
}

// Handle implements the slog.Handler interface.
// It attempts to send the log record to the buffered channel with these behaviors:
// 1. First checks if daemon is shutting down (fast path)
// 2. Then attempts non-blocking channel send
// 3. Returns error if either:
//   - Daemon is shutting down (highest priority)
//   - Channel is full (secondary)
//
// Note: The select statement evaluates both cases simultaneously, so we must
// check ctx.Done() first to ensure proper shutdown behavior.
func (h *BatchHandler) Handle(_ context.Context, r slog.Record) error {
	// Check shutdown first since select is non-sequential
	if h.daemonCtx.Err() != nil {
		return fmt.Errorf("daemon shutting down, dropping log record")
	}

    // Create a new record that includes our stored attributes
    if len(h.attrs) > 0 {
        for _, attr := range h.attrs {
            r.AddAttrs(attr)  // Add our stored attributes to the record
        }
    }

	// Non-blocking channel send attempt
	select {
	case h.recordChan <- r:
		return nil
	default:
		return fmt.Errorf("log channel full, dropping record")
	}
}

func (h *BatchHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    // Create a new handler with combined attributes
    newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
    copy(newAttrs, h.attrs)
    copy(newAttrs[len(h.attrs):], attrs)

    return &BatchHandler{
        configProvider: h.configProvider,
        recordChan:     h.recordChan,
        daemonCtx:      h.daemonCtx,
        attrs:          newAttrs,
    }
}

// WithGroup implements the slog.Handler interface.
// TODO implement
func (h *BatchHandler) WithGroup(name string) slog.Handler {
	return &BatchHandler{
		configProvider: h.configProvider,
		recordChan:     h.recordChan,
		daemonCtx:      h.daemonCtx,
        attrs:          []slog.Attr{}, 
	}
}
