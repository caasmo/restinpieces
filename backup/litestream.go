package backup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/benbjohnson/litestream"
	"github.com/benbjohnson/litestream/file"
)

// Config holds the necessary configuration values for Litestream.
type Config struct {
	DBPath      string // Path to the database file to be backed up.
	ReplicaPath string // Directory path for storing replicas.
	ReplicaName string // Unique identifier for this replica instance.
}

// Litestream handles continuous database backups
type Litestream struct {
	config  Config // Store the specific config
	logger  *slog.Logger
	db      *litestream.DB
	replica *litestream.Replica

	// ctx controls the lifecycle of the backup process
	ctx context.Context

	// cancel stops the backup process
	cancel context.CancelFunc

	// shutdownDone signals when backup has completely stopped
	shutdownDone chan struct{}
}

// NewLitestream creates a new Litestream instance with specific configuration.
func NewLitestream(cfg Config, logger *slog.Logger) (*Litestream, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// --- Database Object ---
	db := litestream.NewDB(cfg.DBPath)
	db.Logger = logger.With("db", cfg.DBPath)

	// --- Replica Client (Assuming File Type) ---
	// Ensure the replica directory exists
	if err := os.MkdirAll(cfg.ReplicaPath, 0750); err != nil && !os.IsExist(err) {
		cancel() // Cancel context if setup fails
		return nil, fmt.Errorf("litestream: failed to create replica directory '%s': %w", cfg.ReplicaPath, err)
	}
	// Get absolute path for the replica client
	absReplicaPath, err := filepath.Abs(cfg.ReplicaPath)
	if err != nil {
		cancel() // Cancel context if setup fails
		return nil, fmt.Errorf("litestream: failed to get absolute replica path for '%s': %w", cfg.ReplicaPath, err)
	}
	replicaClient := file.NewReplicaClient(absReplicaPath)

	// --- Replica Object ---
	replica := litestream.NewReplica(db, cfg.ReplicaName)
	replica.Client = replicaClient
	db.Replicas = append(db.Replicas, replica) // Link replica to DB

	return &Litestream{
		config:         cfg, // Store the provided config
		logger:         logger,
		db:             db,
		replica:        replica,
		ctx:            ctx,
		cancel:         cancel,
		shutdownDone:   make(chan struct{}),
	}, nil
}

// Name returns the name of the service for logging and identification.
func (l *Litestream) Name() string {
	return "LitestreamBackup"
}

// Start begins the continuous backup process in a goroutine.
// It returns an error immediately if the initial setup (opening the database
// or starting the replica) fails. Otherwise, it returns nil and the backup
// process continues in the background.
func (l *Litestream) Start() error {
	// Channel to signal startup completion or error
	startupErrChan := make(chan error, 1)

	go func() {

		defer close(l.shutdownDone) // LIFO last defer
		l.logger.Info("💾 litestream: starting continuous backup")

		// Open database and start monitoring
		if err := l.db.Open(); err != nil {
			l.logger.Error("💾 litestream: failed to open database", "error", err)
			// Signal shutdown immediately on critical error to prevent hanging
			startupErrChan <- err // Report error
			return
		}

		defer func() {
			if err := l.db.Close(); err != nil {
				l.logger.Error("Error closing database during shutdown", "error", err)
			} else {
				l.logger.Debug("Database closed")
			}
		}()

		// Start replication
		if err := l.replica.Start(l.ctx); err != nil {
			l.logger.Error("💾 litestream: failed to start replica", "error", err)
			// Signal shutdown immediately on critical error
			startupErrChan <- err // Report error
			return
		}

		// first defer to execute
		defer func() {
			if err := l.replica.Stop(false); err != nil { // Use false for soft stop
				l.logger.Error("Error stopping replica during shutdown", "error", err)
			} else {
				l.logger.Debug("Replica stopped")
			}
		}()

		l.logger.Info("Replication started successfully")
		startupErrChan <- nil // Signal success

		l.logger.Info("💾 litestream: replication started")
		startupErrChan <- nil // Signal successful startup

		// Wait for shutdown signal
		<-l.ctx.Done()
		l.logger.Info("💾 litestream: received shutdown signal")
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
