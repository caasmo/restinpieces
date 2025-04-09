package server

import (
	"context"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core/proxy"
	"github.com/caasmo/restinpieces/queue/scheduler"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"crypto/tls"
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

func (s *Server) handleSIGHUP() {
	s.logger.Info("Received SIGHUP signal - attempting to reload configuration")
	// you have app. app.Config()
	// we have the flag config in the conf source.
	// TODO: Need the dbfile path here. How to get it?
	// Option 1: Store it in the Server struct.
	// Option 2: Get it from the initial config stored in the provider (if it's there).
	// Let's assume DBFile is in the config for now.
	//	dbFile := s.configProvider.Get().DBFile
	//	if dbFile == "" {
	//		s.logger.Error("Cannot reload config: DBFile path not found in current configuration")
	//		return // Skip reload if path is missing
	//	}
	//	newCfg, err := config.Load(dbFile)
	//	if err != nil {
	//		s.logger.Error("Failed to reload configuration on SIGHUP", "error", err)
	//		// Continue with the old configuration
	//	} else {
	//		s.configProvider.Update(newCfg)
	//		s.logger.Info("Configuration reloaded successfully via SIGHUP")
	//		// Note: Server restart needed for changes in Server config section.
	//	}
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

	s.logServerConfig(&serverCfg)

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
		var err error
		if serverCfg.EnableTLS {
		    srv.TLSConfig = createTLSConfig()
            s.logger.Info("Starting server", "protocol", "HTTPS", "addr", serverCfg.Addr)
			err = srv.ListenAndServeTLS(serverCfg.CertFile, serverCfg.KeyFile)
		} else {
            s.logger.Info("Starting server", "protocol", "HTTP", "addr", serverCfg.Addr)
			err = srv.ListenAndServe()
		}
		if err != http.ErrServerClosed {
			s.logger.Error("ListenAndServe error", "err", err)
			serverError <- err
		}
	}()

	// Start the job scheduler
	s.scheduler.Start()

	// Channel for all signals we want to handle
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGINT,  // kill -SIGINT XXXX or Ctrl+c
		syscall.SIGQUIT, // kill -SIGQUIT XXXX
		syscall.SIGHUP,  // kill -SIGHUP XXXX
	)

	// Wait for signals or server error
	running := true
	for running {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGINT, syscall.SIGQUIT:
				s.logger.Info("Received termination signal - gracefully shutting down", "signal", sig)
				running = false
			case syscall.SIGHUP:
				s.handleSIGHUP()
			}
		case err := <-serverError:
			s.logger.Error("Server error - initiating shutdown", "err", err)
			running = false // Exit the loop
		}
	}

	// Stop listening for signals
	signal.Stop(sigChan)
	close(sigChan)

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

// logServerConfig logs server configuration in a readable format with important settings first
func (s *Server) logServerConfig(cfg *config.Server) {
	s.logger.Info("Server configuration:")
	protocol := "HTTP"
	if cfg.EnableTLS {
		protocol = "HTTPS"
	}
	s.logger.Info("- Protocol", "Protocol", protocol)
	s.logger.Info("- Listening address", "Addr", cfg.Addr)
	if cfg.EnableTLS {
		s.logger.Info("  - Certificate", "CertFile", cfg.CertFile)
		s.logger.Info("  - Private key", "KeyFile", cfg.KeyFile)
	}
	s.logger.Info("- Timeouts:",
		"Read", cfg.ReadTimeout,
		"ReadHeader", cfg.ReadHeaderTimeout,
		"Write", cfg.WriteTimeout,
		"Idle", cfg.IdleTimeout)
	s.logger.Info("- Shutdown grace period", "Timeout", cfg.ShutdownGracefulTimeout)
	if cfg.ClientIpProxyHeader != "" {
		s.logger.Info("- Trusting proxy header", "Header", cfg.ClientIpProxyHeader)
	}
}

// createTLSConfig returns a *tls.Config with secure defaults
func createTLSConfig() *tls.Config {
	return &tls.Config{
		// Force TLS 1.2 as minimum version (1.3 is preferred)
		MinVersion: tls.VersionTLS12,
		
		// Modern cipher suites prioritizing PFS and AEAD
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		
		// Prefer server's cipher suite preference
		PreferServerCipherSuites: true,
		
		// Enable HTTP/2 support
		NextProtos: []string{"h2", "http/1.1"},
		
		// Use only modern elliptic curves
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}
}
