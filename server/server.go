package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/caasmo/restinpieces/config"
	// "github.com/caasmo/restinpieces/core/proxy" // Removed proxy import
	"github.com/caasmo/restinpieces/queue/scheduler"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"net/http"
	"os"
	"time"
	"os/signal"
	"syscall"
)

type Server struct {
	configProvider *config.Provider
	handler        http.Handler
	scheduler      *scheduler.Scheduler
	logger         *slog.Logger
}

func (s *Server) handleSIGHUP() {
	s.logger.Info("Received SIGHUP signal - attempting to reload configuration")
}

// NewServer now accepts any http.Handler.
func NewServer(provider *config.Provider, handler http.Handler, scheduler *scheduler.Scheduler, logger *slog.Logger) *Server {
	return &Server{
		configProvider: provider,
		handler:        handler, // Store the provided handler
		scheduler:      scheduler,
		logger:         logger,
	}
}

func (s *Server) redirectToHTTPS() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get current server config
		serverCfg := s.configProvider.Get().Server
		
		// Construct target URL by combining:
		// - BaseURL() provides the scheme://host:port (always correct format)
		// - RequestURI() provides the path and query (always starts with /, includes ? if query exists)
		// This handles all cases correctly:
		// - Empty path becomes "/" 
		// - Query strings are preserved
		// - Special characters remain properly encoded
		target := serverCfg.BaseURL() + r.URL.RequestURI()
		
		// Perform the redirect with HTTP 301 (Moved Permanently)
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
}

func (s *Server) Run() {
	// Get initial server config
	serverCfg := s.configProvider.Get().Server

	s.logServerConfig(&serverCfg)

	srv := &http.Server{
		Addr:              serverCfg.Addr,
		Handler:           s.handler, // Use the handler field here
		ReadTimeout:       serverCfg.ReadTimeout,
		ReadHeaderTimeout: serverCfg.ReadHeaderTimeout,
		WriteTimeout:      serverCfg.WriteTimeout,
		IdleTimeout:       serverCfg.IdleTimeout,
	}

	var redirectServer *http.Server

	// Start servers
	serverError := make(chan error, 1)
	go func() {
		var err error
		if serverCfg.EnableTLS {
			// Start HTTPS server
			tlsConfig, err := createTLSConfig(&serverCfg)
			if err != nil {
				s.logger.Error("Failed to create TLS config", "error", err)
				serverError <- err
				return
			}
			srv.TLSConfig = tlsConfig
			s.logger.Info("Starting HTTPS server", "addr", serverCfg.Addr)
			
			// Start HTTP->HTTPS redirect server if configured
			if serverCfg.RedirectAddr != "" {
				redirectServer = &http.Server{
					Addr:              serverCfg.RedirectAddr,
					Handler:           s.redirectToHTTPS(),
					ReadTimeout:       1 * time.Second,
					ReadHeaderTimeout: 1 * time.Second,
					WriteTimeout:      1 * time.Second,
					IdleTimeout:       1 * time.Second,
				}

				go func() {
					s.logger.Info("Starting HTTP redirect server", "addr", serverCfg.RedirectAddr)
					if err := redirectServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						serverError <- fmt.Errorf("redirect server error: %w", err)
					}
				}()
			}
			
			err = srv.ListenAndServeTLS("", "")
		} else {
			s.logger.Info("Starting HTTP server", "addr", serverCfg.Addr)
			err = srv.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			s.logger.Error("Server error", "err", err)
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

	// Shutdown main HTTP server in a goroutine
	shutdownGroup.Go(func() error {
		s.logger.Info("Shutting down main HTTP server")
		if err := srv.Shutdown(gracefulCtx); err != nil {
			s.logger.Error("Main HTTP server shutdown error", "err", err)
			return err
		}
		s.logger.Info("Main HTTP server stopped gracefully")
		return nil
	})

	// Shutdown redirect server if it exists
	if redirectServer != nil {
		shutdownGroup.Go(func() error {
			s.logger.Info("Shutting down redirect HTTP server")
			if err := redirectServer.Shutdown(gracefulCtx); err != nil {
				s.logger.Error("Redirect HTTP server shutdown error", "err", err)
				return err
			}
			s.logger.Info("Redirect HTTP server stopped gracefully")
			return nil
		})
	}

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

// logServerConfig logs server configuration with consistent "Server:" prefix
func (s *Server) logServerConfig(cfg *config.Server) {
	protocol := "HTTP"
	if cfg.EnableTLS {
		protocol = "HTTPS"
	}

	s.logger.Info("Server:", "address", cfg.Addr, "protocol", protocol)

	if cfg.EnableTLS {
		if len(cfg.CertData) > 0 && len(cfg.KeyData) > 0 {
			s.logger.Info("Server:", "tls_cert_source", "in-memory_data",
				"cert_data_length", len(cfg.CertData),
				"key_data_length", len(cfg.KeyData))
		} else if cfg.CertFile != "" && cfg.KeyFile != "" {
			s.logger.Info("Server:", "tls_cert_source", "files",
				"cert_file", cfg.CertFile,
				"key_file", cfg.KeyFile)
		} else {
			s.logger.Warn("Server:", "tls_source", "none_configured")
		}
	}

	s.logger.Info("Server:",
		"readTimeout", cfg.ReadTimeout,
		"readHeaderTimeout", cfg.ReadHeaderTimeout,
		"writeTimeout", cfg.WriteTimeout,
		"idleTimeout", cfg.IdleTimeout)

	s.logger.Info("Server:", "ShutdownGracefulTimeout", cfg.ShutdownGracefulTimeout)

	if cfg.ClientIpProxyHeader != "" {
		s.logger.Info("Server:", "header", cfg.ClientIpProxyHeader)
	}
}

// createTLSConfig returns a *tls.Config with secure defaults and certificate data
func createTLSConfig(cfg *config.Server) (*tls.Config, error) {
	var cert tls.Certificate
	var err error

	// Decide which certificate source to use
	if len(cfg.CertData) > 0 && len(cfg.KeyData) > 0 {
		cert, err = tls.X509KeyPair([]byte(cfg.CertData), []byte(cfg.KeyData))
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS key pair from config data: %w", err)
		}
	} else if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err = tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS key pair from files: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no valid TLS certificate configuration found")
	}

	// Create and return the TLS config with the loaded certificate
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,           // Enforce TLS 1.3
		NextProtos:   []string{"h2", "http/1.1"}, // Keep HTTP/2 support
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}, nil
}
