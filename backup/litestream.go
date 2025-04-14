package backup

import (
	"context"
	"log/slog"
	"time"

	"github.com/caasmo/restinpieces/config"
)

// Litestream handles database backups
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

// Start begins the backup process in a goroutine
func (l *Litestream) Start() {
	go func() {
		interval := l.configProvider.Get().Litestream.Interval
		l.logger.Info("💾 litestream: starting", "interval", interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-l.ctx.Done():
				l.logger.Info("💾 litestream: received shutdown signal")
				close(l.shutdownDone)
				return
			case <-ticker.C:
				l.runBackup()
			}
		}
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

// runBackup performs a single backup operation
func (l *Litestream) runBackup() {
	// TODO: Implement actual backup logic using litestream
	// For now just log that we would run a backup
	l.logger.Info("💾 litestream: would perform backup now")
}
