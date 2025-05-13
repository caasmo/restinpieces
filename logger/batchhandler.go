package logger

import (
	"context"
	"log/slog"
	"github.com/caasmo/restinpieces/config"
)

// BatchHandler is a slog.Handler that attempts to send records to a buffered channel.
// The log level is dynamically determined by the AppProvider.
// The channel buffer size is determined by AppProvider.Get().BatchSize at creation time.
// If the channel is full, records are dropped.
type BatchHandler struct {
	provider   *config.Provider 
	recordChan chan slog.Record
}

// NewBatchHandler creates a new BatchHandler.
//
// provider: An instance of your application's configuration provider.
//           It's expected to have a Get() method returning a config object
//           (like *AppConfig) containing LoggerLevel and BatchSize.
//           If provider is nil, or if provider.Get() initially returns a nil config,
//           this function will panic.
//           The BatchSize from the initial config is used to set the channel buffer size.
func NewBatchHandler(provider *AppProvider) *BatchHandler {

	initialConfig := provider.Get()

	batchSize := initialConfig.BatchSize
	// validation to config validate
	//if batchSize < 1 {
	//	batchSize = 1
	//}

	ch := make(chan slog.Record, batchSize)

	return &BatchHandler{
		provider:   provider,
		recordChan: ch,
	}
}

// RecordChan returns the underlying channel used by the handler.
func (h *BatchHandler) RecordChan() <-chan slog.Record {
	return h.recordChan
}

// Enabled implements the slog.Handler interface.
// It consults the AppProvider to get the current logging level.
func (h *BatchHandler) Enabled(_ context.Context, level slog.Level) bool {
	conf := h.provider.Get()
	return level >= conf.LoggerLevel
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
		provider:   h.provider,
		recordChan: h.recordChan,
	}
}

// WithGroup implements the slog.Handler interface.
func (h *BatchHandler) WithGroup(name string) slog.Handler {
	return &BatchHandler{
		provider:   h.provider,
		recordChan: h.recordChan,
	}
}

// Close closes the underlying record channel.
func (h *BatchHandler) Close() {
	if h.recordChan != nil {
		close(h.recordChan)
	}
}
