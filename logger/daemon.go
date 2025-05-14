>package batchsloghandler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

const (
	LoggerDaemonFlushInterval = 1 * time.Second
)

type DBWriter interface {
	WriteLogBatch(ctx context.Context, logs []map[string]any) error
}

type LoggerDaemon struct {
	daemonName string
	handler    *BatchHandler
	dbWriter   DBWriter
	opLogger   *slog.Logger

	batchSize int

	ctx    context.Context
	cancel context.CancelFunc
	// shutdownDone signals when the processing goroutine has completely stopped
	shutdownDone chan struct{}
}

func NewLoggerDaemon(
	daemonName string,
	handler *BatchHandler,
	appProvider *AppProvider,
	dbWriter DBWriter,
	opLogger *slog.Logger,
) (*LoggerDaemon, error) {
	if handler == nil {
		return nil, fmt.Errorf("loggerdaemon: handler cannot be nil")
	}
	if appProvider == nil {
		return nil, fmt.Errorf("loggerdaemon: appProvider cannot be nil")
	}
	if dbWriter == nil {
		return nil, fmt.Errorf("loggerdaemon: dbWriter cannot be nil")
	}
	if opLogger == nil {
		opLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	config := appProvider.Get()
	if config == nil {
		return nil, fmt.Errorf("loggerdaemon: initial config from appProvider unexpectedly nil")
	}

	batchSize := config.BatchSize
	if batchSize < 1 {
		batchSize = 1
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &LoggerDaemon{
		daemonName:   daemonName,
		handler:      handler,
		dbWriter:     dbWriter,
		opLogger:     opLogger.With("daemon_component", "LoggerDaemon", "instance_name", daemonName),
		batchSize:    batchSize,
		ctx:          ctx,
		cancel:       cancel,
		shutdownDone: make(chan struct{}),
	}, nil
}

func (ld *LoggerDaemon) Name() string {
	return ld.daemonName
}

func (ld *LoggerDaemon) Start() error {
	ld.opLogger.Info("Starting LoggerDaemon")
	go ld.processLogs()
	return nil
}

func (ld *LoggerDaemon) Stop(ctx context.Context) error {
	ld.opLogger.Info("Stopping LoggerDaemon")
	ld.cancel()

	select {
	case <-ld.shutdownDone:
		ld.opLogger.Info("LoggerDaemon stopped gracefully")
		return nil
	case <-ctx.Done():
		ld.opLogger.Error("LoggerDaemon shutdown timed out", "error", ctx.Err())
		return ctx.Err()
	}
}

func (ld *LoggerDaemon) processLogs() {
	defer close(ld.shutdownDone)

	ticker := time.NewTicker(LoggerDaemonFlushInterval)
	defer ticker.Stop()

	batch := make([]map[string]any, 0, ld.batchSize)

	flushBatch := func(reason string) {
		if len(batch) == 0 {
			return
		}
		if err := ld.dbWriter.WriteLogBatch(context.Background(), batch); err != nil {
			ld.opLogger.Error("Failed to write log batch to DB", "error", err, "batch_size", len(batch))
		}
		batch = batch[:0]
	}

	for {
		select {
		case record, ok := <-ld.handler.RecordChan():
			if !ok {
				ld.opLogger.Info("BatchHandler's RecordChan closed, flushing final batch and exiting.")
				flushBatch("channel_closed")
				return
			}
			convertedRecord := convertSlogRecordToMap(record)
			batch = append(batch, convertedRecord)
			if len(batch) >= ld.batchSize {
				flushBatch("batch_full")
			}

		case <-ticker.C:
			flushBatch("ticker_flush")

		case <-ld.ctx.Done():
			ld.opLogger.Info("Shutdown signal received, draining remaining logs.")
			flushBatch("shutdown_signal_initial_flush")

		drainLoop:
			for {
				select {
				case record, ok := <-ld.handler.RecordChan():
					if !ok {
						break drainLoop
					}
					convertedRecord := convertSlogRecordToMap(record)
					batch = append(batch, convertedRecord)
					if len(batch) >= ld.batchSize {
						flushBatch("shutdown_drain_batch_full")
					}
				default:
					break drainLoop
				}
			}
			flushBatch("shutdown_final_flush")
			ld.opLogger.Info("LoggerDaemon processing goroutine finished.")
			return
		}
	}
}

func convertSlogRecordToMap(r slog.Record) map[string]any {
	data := make(map[string]any)
	data["time"] = r.Time.UTC().Format(time.RFC3339Nano)
	data["level"] = r.Level.String()
	data["msg"] = r.Message

	r.Attrs(func(a slog.Attr) bool {
		resolveAndInsertAttr(data, a)
		return true
	})
	return data
}

func resolveAndInsertAttr(m map[string]any, a slog.Attr) {
	key := a.Key
	if key == "" {
		return
	}

	val := a.Value.Resolve()

	switch val.Kind() {
	case slog.KindString:
		m[key] = val.String()
	case slog.KindInt64:
		m[key] = val.Int64()
	case slog.KindUint64:
		m[key] = val.Uint64()
	case slog.KindFloat64:
		m[key] = val.Float64()
	case slog.KindBool:
		m[key] = val.Bool()
	case slog.KindDuration:
		m[key] = val.Duration()
	case slog.KindTime:
		m[key] = val.Time().UTC().Format(time.RFC3339Nano)
	case slog.KindGroup:
		groupAttrs := val.Group()
		groupMap := make(map[string]any)
		for _, ga := range groupAttrs {
			resolveAndInsertAttr(groupMap, ga)
		}
		if len(groupMap) > 0 {
			m[key] = groupMap
		}
	default:
		m[key] = val.Any()
	}
}
