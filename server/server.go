package server

import (
	"context"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core/proxy"
	"github.com/caasmo/restinpieces/queue/scheduler"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func Run(cfg config.Server, p *proxy.Proxy, scheduler *scheduler.Scheduler, logger *slog.Logger) {

	logger.Info("Server configuration",
		"addr", cfg.Addr,
		"read_timeout", cfg.ReadTimeout,
		"read_header_timeout", cfg.ReadHeaderTimeout,
		"write_timeout", cfg.WriteTimeout,
		"idle_timeout", cfg.IdleTimeout,
		"shutdown_timeout", cfg.ShutdownGracefulTimeout,
	)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           p,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	// Start HTTP server
	serverError := make(chan error, 1)
	go func() {
		logger.Info("Starting HTTP server", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("ListenAndServe error", "err", err)
			serverError <- err
		}
	}()

	// Start the job scheduler
	scheduler.Start()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGHUP,  // kill -SIGHUP XXXX
		syscall.SIGINT,  // kill -SIGINT XXXX or Ctrl+c
		syscall.SIGQUIT, // kill -SIGQUIT XXXX
	)

	// Wait for either interrupt signal or server error
	select {
	case <-ctx.Done():
		logger.Info("Received shutdown signal - gracefully shutting down")
	case err := <-serverError:
		logger.Error("Server error - initiating shutdown", "err", err)
	}

	// Reset signals default behavior, similar to signal.Reset
	stop()

	gracefulCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownGracefulTimeout)
	defer cancelShutdown()

	// Create a wait group for shutdown tasks
	shutdownGroup, _ := errgroup.WithContext(gracefulCtx)

	// Shutdown HTTP server in a goroutine
	shutdownGroup.Go(func() error {
		logger.Info("Shutting down HTTP server")
		if err := srv.Shutdown(gracefulCtx); err != nil {
			logger.Error("HTTP server shutdown error", "err", err)
			return err
		}
		logger.Info("HTTP server stopped gracefully")
		return nil
	})

	// Shutdown scheduler in a goroutine, passing the graceful context
	shutdownGroup.Go(func() error {
		logger.Info("Shutting down scheduler...")
		if err := scheduler.Stop(gracefulCtx); err != nil {
			logger.Error("Scheduler shutdown error", "err", err)
			return err
		}
		logger.Info("Scheduler stopped gracefully")
		return nil
	})

	// Wait for all shutdown tasks to complete
	if err := shutdownGroup.Wait(); err != nil {
		logger.Error("Error during shutdown", "err", err)
		os.Exit(1)
	}

	logger.Info("All systems stopped gracefully")
	os.Exit(0)

}
