package log

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"zombiezen.com/go/sqlite"
)

// Daemon consumes slog.Records from a channel and writes them to a DB.
// It owns the channel and the database connection.
type Daemon struct {
	// recordChan is owned and managed entirely within Daemon.
	name string // Constant name for this daemon type
	// BatchHandler sends to this channel via the write-end provided by RecordChan().
	recordChan     chan slog.Record
	db             *sqlite.Conn
	opLogger       *slog.Logger
	configProvider *config.Provider

	ctx          context.Context
	cancel       context.CancelFunc
	shutdownDone chan struct{}
}

// New creates a new Daemon.
// It creates a channel for slog.Records and establishes a database connection.
func New(configProvider *config.Provider, opLogger *slog.Logger, db *sqlite.Conn) (*Daemon, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := configProvider.Get()

	if db == nil {
		cancel()
		return nil, fmt.Errorf("logger daemon: database connection cannot be nil")
	}

	daemon := &Daemon{
		name:           "LoggerDaemon", // Constant name for this daemon type
		recordChan:     make(chan slog.Record, cfg.Log.Batch.ChanSize),
		db:             db,
		opLogger:       opLogger.With("daemon_component", "Daemon"),
		configProvider: configProvider,
		ctx:            ctx,
		cancel:         cancel,
		shutdownDone:   make(chan struct{}),
	}
	return daemon, nil
}

// Chan returns the write-end of the channel and the daemon's context.
// The context can be used to check if the daemon is shutting down.
func (ld *Daemon) Chan() (chan<- slog.Record, context.Context) {
	return ld.recordChan, ld.ctx
}

// Name returns the constant name of this daemon type.
func (ld *Daemon) Name() string {
	return "LoggerDaemon"
}

// Start begins the daemon's log processing goroutine.
func (ld *Daemon) Start() error {
	ld.opLogger.Info("Starting Daemon's processing goroutine")
	go ld.processLogs()
	return nil
}

// Stop gracefully shuts down the daemon.
func (ld *Daemon) Stop(ctx context.Context) error {
	ld.opLogger.Info("Stopping Daemon")
	ld.cancel()

	select {
	case <-ld.shutdownDone:
		ld.opLogger.Info("Daemon processing goroutine confirmed shutdown.")
	case <-ctx.Done():
		ld.opLogger.Error("Daemon shutdown timed out waiting for processing goroutine", "error", ctx.Err())
		return ctx.Err()
	}

	ld.opLogger.Info("Daemon stopped gracefully.")
	return nil
}

// prepareRecordForDB converts an slog.Record into a dbLogEntry using structured types
func (ld *Daemon) prepareRecordForDB(record slog.Record) (db.Log, error) {
	logObj, err := schema.FromRecord(record)
	if err != nil {
		return db.Log{}, fmt.Errorf("failed to convert record: %w", err)
	}

	jsonDataBytes, err := json.Marshal(logObj)
	if err != nil {
		return db.Log{}, fmt.Errorf("failed to marshal log object: %w", err)
	}

	return db.Log{
		Level:    int64(record.Level.Level()),
		Message:  record.Message,
		JsonData: string(jsonDataBytes),
		Created:  record.Time.UTC().Format(time.RFC3339Nano),
	}, nil
}

// processLogs is the internal goroutine that reads from the channel, prepares, and writes to the DB.
func (ld *Daemon) processLogs() {
	defer close(ld.shutdownDone)

	cfg := ld.configProvider.Get()
	ticker := time.NewTicker(cfg.Log.Batch.FlushInterval.Duration)
	defer ticker.Stop()

	batch := make([]db.Log, 0, cfg.Log.Batch.FlushSize)

	flushBatch := func(reason string) {
		if len(batch) == 0 {
			return
		}
		if err := zombiezen.WriteLogBatch(ld.db, batch); err != nil {
			ld.opLogger.Error("Failed to write log batch to DB", "error", err, "batch_size", len(batch), "reason", reason)
		}
		batch = batch[:0]
	}

	for {
		select {
		case record, ok := <-ld.recordChan:
			if !ok {
				ld.opLogger.Info("Record channel closed by owner, exiting processLogs.")
				flushBatch("channel_closed_by_owner")
				return
			}

			dbEntry, err := ld.prepareRecordForDB(record)
			if err != nil {
				// Log the error with relevant details from the original record.
				// The full record.Message is now passed.
				ld.opLogger.Error("Failed to prepare record for DB, skipping",
					"error", err, "record_time", record.Time, "record_msg", record.Message)
				continue
			}

			batch = append(batch, dbEntry)
			if len(batch) >= cfg.Log.Batch.FlushSize {
				flushBatch("db_batch_full")
			}

		case <-ticker.C:
			flushBatch("ticker_flush")

		case <-ld.ctx.Done():
			ld.opLogger.Info("Shutdown signal (ctx.Done) received, draining remaining logs from channel.")
		drainLoop:
			for {
				select {
				case record, ok := <-ld.recordChan:
					if !ok {
						ld.opLogger.Info("Record channel closed during drain.")
						break drainLoop
					}
					dbEntry, err := ld.prepareRecordForDB(record)
					if err != nil {
						ld.opLogger.Error("Failed to prepare record during drain, skipping",
							"error", err, "record_time", record.Time, "record_msg", record.Message)
						continue
					}
					batch = append(batch, dbEntry)
					if len(batch) >= cfg.Log.Batch.FlushSize {
						flushBatch("shutdown_drain_db_batch_full")
					}
				default:
					ld.opLogger.Debug("Record channel empty during drain.")
					break drainLoop
				}
			}
			flushBatch("shutdown_final_flush")
			ld.opLogger.Info("Daemon processing goroutine finished draining")
			ld.opLogger.Info("Closing owned record channel.")
			close(ld.recordChan)

			ld.opLogger.Info("Closing database connection.")
			if ld.db != nil {
				if err := ld.db.Close(); err != nil {
					ld.opLogger.Error("Failed to close database connection", "error", err)
				}
			}
			return
		}
	}
}

// WriteLogBatch writes a batch of pre-processed dbLogEntry items to the SQLite database.
func (ld *Daemon) WriteLogBatch(ctx context.Context, batch []db.Log) error {
	return zombiezen.WriteLogBatch(ld.db, batch)
}

// convertSlogRecordToMap is primarily used by resolveAndInsertAttr for group attributes.
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

// resolveAndInsertAttr recursively resolves attributes and adds them to the map.
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
		m[key] = val.Duration().String()
	case slog.KindTime:
		m[key] = val.Time().UTC().Format(time.RFC3339Nano)
	case slog.KindGroup:
		groupAttrs := val.Group()
		if len(groupAttrs) == 0 {
			return
		}
		groupMap := make(map[string]any)
		for _, ga := range groupAttrs {
			resolveAndInsertAttr(groupMap, ga)
		}
		if len(groupMap) > 0 {
			m[key] = groupMap
		}
	default:
		anyVal := val.Any()
		switch v := anyVal.(type) {
		case error:
			m[key] = v.Error()
		default:
			m[key] = fmt.Sprint(anyVal)
		}
	}
}
