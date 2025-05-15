package log

import (
	"context"
	"github.com/caasmo/restinpieces/config"
	"log/slog"
)

// BatchHandler is a slog.Handler that attempts to send records to an externally provided channel.
// The log level is dynamically determined by the config provider.
// If the channel is full, records are dropped.
type BatchHandler struct {
	configProvider *config.Provider   // For dynamic log levels
	recordChan     chan<- slog.Record // Write-end of the channel, provided by Daemon
}

// NewBatchHandler creates a new BatchHandler.
//
// configProvider: An instance of the configuration provider for dynamic log levels.
// recordChan: The write-end of a buffered channel where slog.Records will be sent.
//             This channel is created and managed by Daemon.
// If configProvider or recordChan is nil, this function will panic.
func NewBatchHandler(configProvider *config.Provider, recordChan chan<- slog.Record) *BatchHandler {
	if configProvider == nil {
		panic("batchhandler: configProvider cannot be nil")
	}
	if recordChan == nil {
		panic("batchhandler: recordChan cannot be nil")
	}

	return &BatchHandler{
		configProvider: configProvider,
		recordChan:     recordChan,
	}
}

// Enabled implements the slog.Handler interface.
// It consults the config provider to get the current logging level.
func (h *BatchHandler) Enabled(_ context.Context, level slog.Level) bool {
	conf := h.configProvider.Get()
	return level >= conf.LoggerBatch.Level.Level
}

// Handle implements the slog.Handler interface.
// It attempts to send the log record to the buffered channel in a non-blocking way.
// If the channel is nil or full, the record is dropped and the method returns.
func (h *BatchHandler) Handle(_ context.Context, r slog.Record) error {

	select {
	case h.recordChan <- r:
	default:
		// Channel is full, record is dropped
	}
	return nil
}

// WithAttrs implements the slog.Handler interface.
func (h *BatchHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &BatchHandler{
		configProvider: h.configProvider,
		recordChan:     h.recordChan,
	}
}

// WithGroup implements the slog.Handler interface.
func (h *BatchHandler) WithGroup(name string) slog.Handler {
	return &BatchHandler{
		configProvider: h.configProvider,
		recordChan:     h.recordChan,
	}
}
