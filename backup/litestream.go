package backup

import (
	"context"
	"log/slog"


    "github.com/benbjohnson/litestream"
    "github.com/benbjohnson/litestream/file"
    "github.com/caasmo/restinpieces/config"
)

// Litestream handles continuous database backups
type Litestream struct {
	configProvider *config.Provider
	logger         *slog.Logger

	// ctx controls the lifecycle of the backup process
	ctx context.Context

	// cancel stops the backup process
	cancel context.CancelFunc

	// shutdownDone signals when backup has completely stopped
	shutdownDone chan struct{}
}

func NewLitestream(configProvider *config.Provider, logger *slog.Logger) *Litestream {
	ctx, cancel := context.WithCancel(context.Background())
	return &Litestream{
		configProvider: configProvider,
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
		shutdownDone:   make(chan struct{}),
	}
}

// Start begins the continuous backup process in a goroutine
func (l *Litestream) Start() {
	go func() {
		l.logger.Info("💾 litestream: starting continuous backup")
		defer close(l.shutdownDone)

		if err := l.run(); err != nil {
			l.logger.Error("💾 litestream: failed to run", "error", err)
		}
		l.logger.Info("💾 litestream: received shutdown signal")
	}()
}

// Stop gracefully shuts down the backup process
func (l *Litestream) Stop(ctx context.Context) error {
	l.logger.Info("💾 litestream: stopping")
	l.cancel()

	select {
	case <-l.shutdownDone:
		l.logger.Info("💾 litestream: stopped gracefully")
		return nil
	case <-ctx.Done():
		l.logger.Info("💾 litestream: shutdown timed out")
		return ctx.Err()
	}
}

// run implements the continuous litestream backup process
func (l *Litestream) run() error {
    cfg := l.configProvider.Get().Litestream

    // Create and configure the database object
    db := litestream.NewDB(cfg.DBPath)
    db.Logger = l.logger.With("db", cfg.DBPath)

    // Create replica client based on config
    var replicaClient litestream.ReplicaClient
    switch cfg.ReplicaType {
    case "file":
        if err := os.MkdirAll(cfg.ReplicaPath, 0750); err != nil && !os.IsExist(err) {
            return fmt.Errorf("failed to create replica directory: %w", err)
        }
        absPath, err := filepath.Abs(cfg.ReplicaPath)
        if err != nil {
            return fmt.Errorf("failed to get absolute replica path: %w", err)
        }
        replicaClient = file.NewReplicaClient(absPath)
    default:
        return fmt.Errorf("unsupported replica type: %s", cfg.ReplicaType)
    }

    // Create and configure replica
    replica := litestream.NewReplica(db, cfg.ReplicaName)
    replica.Client = replicaClient

    // Open database and start monitoring
    if err := db.Open(); err != nil {
        return fmt.Errorf("failed to open database: %w", err)
    }
    defer db.Close()

    // Start replication
    if err := replica.Start(l.ctx); err != nil {
        return fmt.Errorf("failed to start replica: %w", err)
    }

    // Wait for shutdown signal
    <-l.ctx.Done()

    // Stop replica gracefully
    if err := replica.Stop(false); err != nil {
        return fmt.Errorf("error stopping replica: %w", err)
    }

    return nil
}
