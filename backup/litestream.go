package backup

import (
	"context"
	"log/slog"

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

		// TODO: Implement continuous backup using litestream
		// This will block until ctx is canceled
		<-l.ctx.Done()
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
