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
	db             *litestream.DB
	replica        *litestream.Replica

	// ctx controls the lifecycle of the backup process
	ctx context.Context

	// cancel stops the backup process
	cancel context.CancelFunc

	// shutdownDone signals when backup has completely stopped
	shutdownDone chan struct{}
}

func NewLitestream(configProvider *config.Provider, logger *slog.Logger) (*Litestream, error) {
	cfg := configProvider.Get().Litestream
	ctx, cancel := context.WithCancel(context.Background())

	// Create and configure the database object
	db := litestream.NewDB(cfg.DBPath)
	db.Logger = logger.With("db", cfg.DBPath)

	// Create replica client based on config
	var replicaClient litestream.ReplicaClient
	switch cfg.ReplicaType {
	case "file":
		if err := os.MkdirAll(cfg.ReplicaPath, 0750); err != nil && !os.IsExist(err) {
			return nil, fmt.Errorf("failed to create replica directory: %w", err)
		}
		absPath, err := filepath.Abs(cfg.ReplicaPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute replica path: %w", err)
		}
		replicaClient = file.NewReplicaClient(absPath)
	default:
		return nil, fmt.Errorf("unsupported replica type: %s", cfg.ReplicaType)
	}

	// Create and configure replica
	replica := litestream.NewReplica(db, cfg.ReplicaName)
	replica.Client = replicaClient

	return &Litestream{
		configProvider: configProvider,
		logger:         logger,
		db:             db,
		replica:        replica,
		ctx:            ctx,
		cancel:         cancel,
		shutdownDone:   make(chan struct{}),
	}, nil
}

// Start begins the continuous backup process in a goroutine
// Start begins the continuous backup process in a goroutine
func (l *Litestream) Start() {
	go func() {
		l.logger.Info("💾 litestream: starting continuous backup")

		// Open database and start monitoring
		if err := l.db.Open(); err != nil {
			l.logger.Error("💾 litestream: failed to open database", "error", err)
			// Signal shutdown immediately on critical error to prevent hanging
			close(l.shutdownDone)
			return
		}
		defer l.db.Close()

		// Start replication
		if err := l.replica.Start(l.ctx); err != nil {
			l.logger.Error("💾 litestream: failed to start replica", "error", err)
			// Signal shutdown immediately on critical error
			close(l.shutdownDone)
			return
		}

		l.logger.Info("💾 litestream: replication started")

		// Wait for shutdown signal
		<-l.ctx.Done()
		l.logger.Info("💾 litestream: received shutdown signal")

		// Stop replica gracefully
		if err := l.replica.Stop(false); err != nil {
			l.logger.Error("💾 litestream: error stopping replica", "error", err)
		}
		close(l.shutdownDone)
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
