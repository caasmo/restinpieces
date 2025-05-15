package log

import (
	"context"
	"fmt"
	"github.com/caasmo/restinpieces/config"
	"log/slog"
)

// BatchHandler is a lightweight slog.Handler that sends records to a channel for batched processing.
// Important implementation notes:
// - Error handling is lightweight - failed logs are simply dropped with an error return
// - The select statement in Handle() is not sequential - both cases are evaluated simultaneously
// - Checking ctx.Done() first is crucial to avoid sending during shutdown
// - Channel writes are non-blocking - full channels result in dropped logs
// - Designed for high throughput at the cost of some reliability
type BatchHandler struct {
	configProvider *config.Provider   // For dynamic log levels
	recordChan     chan<- slog.Record // Write-end of the channel, provided by Daemon
	daemonCtx      context.Context    // Context from daemon for shutdown detection
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
	}
}

// Enabled implements the slog.Handler interface.
// It consults the config provider to get the current logging level.
func (h *BatchHandler) Enabled(_ context.Context, level slog.Level) bool {
	conf := h.configProvider.Get()
	return level >= conf.LoggerBatch.Level.Level
}

// Handle implements the slog.Handler interface.
// It attempts to send the log record to the buffered channel with these behaviors:
// 1. First checks if daemon is shutting down (fast path)
// 2. Then attempts non-blocking channel send
// 3. Returns error if either:
//    - Daemon is shutting down (highest priority)
//    - Channel is full (secondary)
//
// Note: The select statement evaluates both cases simultaneously, so we must
// check ctx.Done() first to ensure proper shutdown behavior.
func (h *BatchHandler) Handle(_ context.Context, r slog.Record) error {
    // Check shutdown first since select is non-sequential
    if h.daemonCtx.Err() != nil {
        return fmt.Errorf("daemon shutting down, dropping log record")
    }
    
    // Non-blocking channel send attempt
    select {
    case h.recordChan <- r:
        return nil
    default:
        return fmt.Errorf("log channel full, dropping record")
    }
}

// WithAttrs implements the slog.Handler interface.
func (h *BatchHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &BatchHandler{
		configProvider: h.configProvider,
		recordChan:     h.recordChan,
		daemonCtx:      h.daemonCtx,
	}
}

// WithGroup implements the slog.Handler interface.
func (h *BatchHandler) WithGroup(name string) slog.Handler {
	return &BatchHandler{
		configProvider: h.configProvider,
		recordChan:     h.recordChan,
		daemonCtx:      h.daemonCtx,
	}
}
