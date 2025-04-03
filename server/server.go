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

type Server struct {
	cfg      config.Server
	proxy    *proxy.Proxy
	scheduler *scheduler.Scheduler
	logger   *slog.Logger
}

func NewServer(cfg config.Server, p *proxy.Proxy, scheduler *scheduler.Scheduler, logger *slog.Logger) *Server {
	return &Server{
		cfg:      cfg,
		proxy:    p,
		scheduler: scheduler,
		logger:   logger,
	}
}

func (s *Server) Run() {

	s.logger.Info("Server configuration",
		"addr", s.cfg.Addr,
		"read_timeout", s.cfg.ReadTimeout,
		"read_header_timeout", s.cfg.ReadHeaderTimeout,
		"write_timeout", s.cfg.WriteTimeout,
		"idle_timeout", s.cfg.IdleTimeout,
		"shutdown_timeout", s.cfg.ShutdownGracefulTimeout,
	)

	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.proxy,
		ReadTimeout:       s.cfg.ReadTimeout,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		WriteTimeout:      s.cfg.WriteTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
	}

	// Start HTTP server
	serverError := make(chan error, 1)
	go func() {
		s.logger.Info("Starting HTTP server", "addr", s.cfg.Addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Error("ListenAndServe error", "err", err)
			serverError <- err
		}
	}()

	// Start the job scheduler
	s.scheduler.Start()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGHUP,  // kill -SIGHUP XXXX
		syscall.SIGINT,  // kill -SIGINT XXXX or Ctrl+c
		syscall.SIGQUIT, // kill -SIGQUIT XXXX
	)

	// Wait for either interrupt signal or server error
	select {
	case <-ctx.Done():
		s.logger.Info("Received shutdown signal - gracefully shutting down")
	case err := <-serverError:
		s.logger.Error("Server error - initiating shutdown", "err", err)
	}

	// Reset signals default behavior, similar to signal.Reset
	stop()

	gracefulCtx, cancelShutdown := context.WithTimeout(context.Background(), s.cfg.ShutdownGracefulTimeout)
	defer cancelShutdown()

	// Create a wait group for shutdown tasks
	shutdownGroup, _ := errgroup.WithContext(gracefulCtx)

	// Shutdown HTTP server in a goroutine
	shutdownGroup.Go(func() error {
		s.logger.Info("Shutting down HTTP server")
		if err := srv.Shutdown(gracefulCtx); err != nil {
			s.logger.Error("HTTP server shutdown error", "err", err)
			return err
		}
		s.logger.Info("HTTP server stopped gracefully")
		return nil
	})

	// Shutdown scheduler in a goroutine, passing the graceful context
	shutdownGroup.Go(func() error {
		s.logger.Info("Shutting down scheduler...")
		if err := s.scheduler.Stop(gracefulCtx); err != nil {
			s.logger.Error("Scheduler shutdown error", "err", err)
			return err
		}
		s.logger.Info("Scheduler stopped gracefully")
		return nil
	})

	// Wait for all shutdown tasks to complete
	if err := shutdownGroup.Wait(); err != nil {
		s.logger.Error("Error during shutdown", "err", err)
		os.Exit(1)
	}

	s.logger.Info("All systems stopped gracefully")
	os.Exit(0)

}
