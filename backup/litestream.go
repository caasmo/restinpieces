package backup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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
	mainCfg := configProvider.Get()
	litestreamCfg := mainCfg.Litestream
	ctx, cancel := context.WithCancel(context.Background())

	// --- Database Object ---
	// Use the main DBPath from the overall config
	db := litestream.NewDB(mainCfg.DBPath)
	db.Logger = logger.With("db", mainCfg.DBPath)

	// --- Replica Client (Assuming File Type) ---
	// Ensure the replica directory exists
	if err := os.MkdirAll(litestreamCfg.ReplicaPath, 0750); err != nil && !os.IsExist(err) {
		cancel() // Cancel context if setup fails
		return nil, fmt.Errorf("litestream: failed to create replica directory '%s': %w", litestreamCfg.ReplicaPath, err)
	}
	// Get absolute path for the replica client
	absReplicaPath, err := filepath.Abs(litestreamCfg.ReplicaPath)
	if err != nil {
		cancel() // Cancel context if setup fails
		return nil, fmt.Errorf("litestream: failed to get absolute replica path for '%s': %w", litestreamCfg.ReplicaPath, err)
	}
	replicaClient := file.NewReplicaClient(absReplicaPath)

	// --- Replica Object ---
	// Use the ReplicaName from the Litestream config section
	replica := litestream.NewReplica(db, litestreamCfg.ReplicaName)
	replica.Client = replicaClient
	db.Replicas = append(db.Replicas, replica) // Link replica to DB

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

// Start begins the continuous backup process in a goroutine.
// It returns an error immediately if the initial setup (opening the database
// or starting the replica) fails. Otherwise, it returns nil and the backup
// process continues in the background.
func (l *Litestream) Start() error {
	// Channel to signal startup completion or error
	startupErrChan := make(chan error, 1)

	go func() {
		l.logger.Info("💾 litestream: starting continuous backup")

		// Open database and start monitoring
		if err := l.db.Open(); err != nil {
			l.logger.Error("💾 litestream: failed to open database", "error", err)
			// Signal shutdown immediately on critical error to prevent hanging
			close(l.shutdownDone)
			startupErrChan <- err // Report error
			return
		}
		// defer l.db.Close() // Removed defer

		// Start replication
		if err := l.replica.Start(l.ctx); err != nil {
			l.logger.Error("💾 litestream: failed to start replica", "error", err)
			// Signal shutdown immediately on critical error
			close(l.shutdownDone)
			startupErrChan <- err // Report error
			return
		}

		l.logger.Info("💾 litestream: replication started")
		startupErrChan <- nil // Signal successful startup

		// Wait for shutdown signal
		<-l.ctx.Done()
		l.logger.Info("💾 litestream: received shutdown signal")

		// Stop replica gracefully
		if err := l.replica.Stop(false); err != nil {
			l.logger.Error("💾 litestream: error stopping replica", "error", err)
		}

		// Explicitly close the database *before* signaling shutdown completion
		if err := l.db.Close(); err != nil {
			l.logger.Error("💾 litestream: error closing database", "error", err)
		}

		close(l.shutdownDone) // Now signal that shutdown is fully complete
	}()

	// Wait for the goroutine to signal startup completion or error
	err := <-startupErrChan
	return err
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
