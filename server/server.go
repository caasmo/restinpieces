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
	configProvider *config.Provider
	proxy          *proxy.Proxy
	scheduler      *scheduler.Scheduler
	logger         *slog.Logger
}

func NewServer(provider *config.Provider, p *proxy.Proxy, scheduler *scheduler.Scheduler, logger *slog.Logger) *Server {
	return &Server{
		configProvider: provider,
		proxy:          p,
		scheduler:      scheduler,
		logger:         logger,
	}
}

func (s *Server) Run() {
	// Get initial server config
	serverCfg := s.configProvider.Get().Server

	s.logger.Info("Server configuration",
		"addr", serverCfg.Addr,
		"read_timeout", serverCfg.ReadTimeout,
		"read_header_timeout", serverCfg.ReadHeaderTimeout,
		"write_timeout", serverCfg.WriteTimeout,
		"idle_timeout", serverCfg.IdleTimeout,
		"shutdown_timeout", serverCfg.ShutdownGracefulTimeout,
	)

	srv := &http.Server{
		Addr:              serverCfg.Addr,
		Handler:           s.proxy,
		ReadTimeout:       serverCfg.ReadTimeout,
		ReadHeaderTimeout: serverCfg.ReadHeaderTimeout,
		WriteTimeout:      serverCfg.WriteTimeout,
		IdleTimeout:       serverCfg.IdleTimeout,
	}

	// Start HTTP server
	serverError := make(chan error, 1)
	go func() {
		// Use the Addr from the initial config used to create the server
		s.logger.Info("Starting HTTP server", "addr", serverCfg.Addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Error("ListenAndServe error", "err", err)
			serverError <- err
		}
	}()

	// Start the job scheduler
	s.scheduler.Start()

	// Channel for termination signals (SIGINT, SIGQUIT)
	ctxTerminate, stopTerminate := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,  // kill -SIGINT XXXX or Ctrl+c
		syscall.SIGQUIT, // kill -SIGQUIT XXXX
	)

	// Channel specifically for SIGHUP
	reloadChan := make(chan os.Signal, 1)
	signal.Notify(reloadChan, syscall.SIGHUP) // kill -SIGHUP XXXX

	// Wait for termination, SIGHUP, or server error
	running := true
	for running {
		select {
		case <-ctxTerminate.Done():
			s.logger.Info("Received termination signal - gracefully shutting down")
			running = false // Exit the loop
		case <-reloadChan:
			s.logger.Info("Received SIGHUP signal - attempting to reload configuration")
			// TODO: Need the dbfile path here. How to get it?
			// Option 1: Store it in the Server struct.
			// Option 2: Get it from the initial config stored in the provider (if it's there).
			// Let's assume DBFile is in the config for now.
			dbFile := s.configProvider.Get().DBFile
			if dbFile == "" {
				s.logger.Error("Cannot reload config: DBFile path not found in current configuration")
				continue // Skip reload if path is missing
			}
			newCfg, err := config.Load(dbFile)
			if err != nil {
				s.logger.Error("Failed to reload configuration on SIGHUP", "error", err)
				// Continue with the old configuration
			} else {
				s.configProvider.Update(newCfg)
				s.logger.Info("Configuration reloaded successfully via SIGHUP")
				// Note: Server restart needed for changes in Server config section.
			}
		case err := <-serverError:
			s.logger.Error("Server error - initiating shutdown", "err", err)
			running = false // Exit the loop
		}
	}

	// Stop listening for signals
	stopTerminate()
	signal.Stop(reloadChan)
	close(reloadChan)

	// Get shutdown timeout from the *current* config
	shutdownTimeout := serverCfg.ShutdownGracefulTimeout
	gracefulCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
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
