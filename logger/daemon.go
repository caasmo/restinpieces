package logger

const (
	LoggerDaemonFlushInterval = 1 * time.Second
)

// DBWriter is an interface for writing log batches to a database.
type DBWriter interface {
	WriteLogBatch(ctx context.Context, logs []map[string]any) error
}

// LoggerDaemon consumes slog.Records from an internal channel and writes them to a DB.
// It owns the internal channel.
type LoggerDaemon struct {
	daemonName string
	// internalRecordChan is owned and managed entirely within LoggerDaemon.
	// BatchHandler sends to this channel via the write-end provided by RecordInputChan().
	internalRecordChan chan slog.Record
	dbWriter           DBWriter
	opLogger           *slog.Logger
	appProvider        *AppProvider // For batch sizes

	// dbBatchSize is for flushing to DB, derived from AppProvider.Get().BatchSize
	dbBatchSize int

	ctx          context.Context
	cancel       context.CancelFunc
	shutdownDone chan struct{}
}

// NewLoggerDaemon creates a new LoggerDaemon.
// It creates an internal channel for slog.Records.
// The write-end of this channel can be retrieved via RecordInputChan().
func NewLoggerDaemon(
	daemonName string,
	appProvider *AppProvider,
	dbWriter DBWriter,
	opLogger *slog.Logger,
) (*LoggerDaemon, error) {
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

	channelBufferSize := config.BatchSize
	if channelBufferSize < 1 {
		channelBufferSize = 1
	}
	dbBatchSize := config.BatchSize
	if dbBatchSize < 1 {
		dbBatchSize = 1
	}

	ctx, cancel := context.WithCancel(context.Background())

	daemon := &LoggerDaemon{
		daemonName:         daemonName,
		internalRecordChan: make(chan slog.Record, channelBufferSize), // Creates and owns this channel
		dbWriter:           dbWriter,
		opLogger:           opLogger.With("daemon_component", "LoggerDaemon", "instance_name", daemonName),
		appProvider:        appProvider,
		dbBatchSize:        dbBatchSize,
		ctx:                ctx,
		cancel:             cancel,
		shutdownDone:       make(chan struct{}),
	}
	return daemon, nil
}

// RecordInputChan returns the write-end of the internal channel.
// This is intended to be used by BatchHandler to send records to this daemon.
func (ld *LoggerDaemon) RecordInputChan() chan<- slog.Record {
	return ld.internalRecordChan
}

// Name returns the name of the daemon.
func (ld *LoggerDaemon) Name() string {
	return ld.daemonName
}

// Start begins the daemon's log processing goroutine.
func (ld *LoggerDaemon) Start() error {
	ld.opLogger.Info("Starting LoggerDaemon's processing goroutine")
	go ld.processLogs()
	return nil
}

// Stop gracefully shuts down the daemon.
// It signals the processing goroutine, waits for it to drain and finish,
// and then closes the internal record channel.
func (ld *LoggerDaemon) Stop(ctx context.Context) error {
	ld.opLogger.Info("Stopping LoggerDaemon")
	ld.cancel() // Signal the processLogs goroutine to stop

	select {
	case <-ld.shutdownDone:
		ld.opLogger.Info("LoggerDaemon processing goroutine confirmed shutdown.")
	case <-ctx.Done():
		ld.opLogger.Error("LoggerDaemon shutdown timed out waiting for processing goroutine", "error", ctx.Err())
		// Even if timed out, the channel closure attempt is important.
		// However, processLogs might still be running, leading to a panic if it reads after close.
		// The shutdownDone signal is crucial. If timeout happens, it implies a problem in processLogs' exit.
		return ctx.Err()
	}

	// At this point, shutdownDone is closed, meaning processLogs has exited.
	// It is now safe for the owner (LoggerDaemon) to close its internal channel.
	ld.opLogger.Info("Closing owned internal record channel.")
	close(ld.internalRecordChan)

	ld.opLogger.Info("LoggerDaemon stopped gracefully.")
	return nil
}

// processLogs is the internal goroutine that reads from the channel and writes to the DB.
func (ld *LoggerDaemon) processLogs() {
	defer close(ld.shutdownDone) // Signal that this goroutine has finished

	ticker := time.NewTicker(LoggerDaemonFlushInterval)
	defer ticker.Stop()

	batch := make([]map[string]any, 0, ld.dbBatchSize)

	flushBatch := func(reason string) {
		if len(batch) == 0 {
			return
		}
		// Using a background context for DB write.
		if err := ld.dbWriter.WriteLogBatch(context.Background(), batch); err != nil {
			ld.opLogger.Error("Failed to write log batch to DB", "error", err, "batch_size", len(batch), "reason", reason)
		}
		batch = batch[:0] // Reset batch
	}

	for {
		select {
		case record, ok := <-ld.internalRecordChan:
			if !ok {
				// This occurs when ld.internalRecordChan is closed by ld.Stop().
				// This is the expected way for this loop to terminate after ctx.Done()
				// has caused the drain and Stop() proceeds to close the channel.
				ld.opLogger.Info("Internal record channel closed by owner, exiting processLogs.")
				flushBatch("channel_closed_by_owner") // Final flush
				return
			}
			convertedRecord := convertSlogRecordToMap(record)
			batch = append(batch, convertedRecord)
			if len(batch) >= ld.dbBatchSize {
				flushBatch("db_batch_full")
			}

		case <-ticker.C:
			flushBatch("ticker_flush")

		case <-ld.ctx.Done(): // Primary shutdown signal
			ld.opLogger.Info("Shutdown signal (ctx.Done) received, draining remaining logs from channel.")
		drainLoop: // Drain the channel after ctx is cancelled but before channel is closed by Stop().
			for {
				select {
				case record, ok := <-ld.internalRecordChan:
					if !ok {
						// Channel was closed (likely by Stop() if shutdown was very fast, or an error occurred)
						ld.opLogger.Info("Internal record channel closed during drain.")
						break drainLoop
					}
					convertedRecord := convertSlogRecordToMap(record)
					batch = append(batch, convertedRecord)
					if len(batch) >= ld.dbBatchSize {
						flushBatch("shutdown_drain_db_batch_full")
					}
				default: // Channel is empty at this moment
					ld.opLogger.Debug("Internal record channel empty during drain.")
					break drainLoop
				}
			}
			flushBatch("shutdown_final_flush") // Flush any remaining items in the batch
			ld.opLogger.Info("LoggerDaemon processing goroutine finished draining, awaiting channel close by Stop().")
			// This goroutine will now block on `<-ld.internalRecordChan` until Stop() closes it,
			// at which point `ok` will be `false` and it will exit.
			// Or, if already empty and Stop() closes it immediately, it will exit directly.
			// The `return` is handled by the `!ok` case of the main channel read.
		}
	}
}

// convertSlogRecordToMap converts a slog.Record to a map for DB storage.
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
	if key == "" { // Skip empty keys
		return
	}

	val := a.Value.Resolve() // Resolve LogValuers

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
		m[key] = val.Duration().String() // Store as string for broad DB compatibility
	case slog.KindTime:
		m[key] = val.Time().UTC().Format(time.RFC3339Nano)
	case slog.KindGroup:
		groupAttrs := val.Group()
		if len(groupAttrs) == 0 { // Don't add empty groups
			return
		}
		groupMap := make(map[string]any)
		for _, ga := range groupAttrs {
			resolveAndInsertAttr(groupMap, ga)
		}
		if len(groupMap) > 0 { // Only add group if it has content
			m[key] = groupMap
		}
	default: // slog.KindAny or other (after Resolve)
		// Attempt to represent common types, otherwise stringify
		anyVal := val.Any()
		switch v := anyVal.(type) {
		case error:
			m[key] = v.Error() // Store error as string
		default:
			m[key] = fmt.Sprint(anyVal) // Fallback to string representation
		}
	}
}
