package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/caasmo/restinpieces/config"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Daemon defines the contract for background components managed
// by the server's lifecycle (Start/Stop).
type Daemon interface {
	Name() string // For logging/identification
	Start() error
	Stop(ctx context.Context) error
}

type Server struct {
	configProvider *config.Provider
	handler        http.Handler // The main HTTP handler
	logger         *slog.Logger
	daemons        []Daemon // Collection of managed daemons
}

func (s *Server) handleSIGHUP() {
	s.logger.Info("Received SIGHUP signal - attempting to reload configuration")
}

// NewServer constructor - daemons are added via AddDaemon.
func NewServer(provider *config.Provider, handler http.Handler, logger *slog.Logger) *Server {
	return &Server{
		configProvider: provider,
		handler:        handler,
		logger:         logger,
		daemons:        make([]Daemon, 0), // Initialize empty slice
	}
}

// AddDaemon adds a daemon whose lifecycle will be managed by the server.
func (s *Server) AddDaemon(daemon Daemon) {
	if daemon == nil {
		s.logger.Warn("Attempted to add a nil daemon")
		return
	}
	s.logger.Info("Adding daemon", "daemon_name", daemon.Name())
	s.daemons = append(s.daemons, daemon)
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
		ReadTimeout:       serverCfg.ReadTimeout.Duration,
		ReadHeaderTimeout: serverCfg.ReadHeaderTimeout.Duration,
		WriteTimeout:      serverCfg.WriteTimeout.Duration,
		IdleTimeout:       serverCfg.IdleTimeout.Duration,
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
					ReadTimeout:       time.Second,
					ReadHeaderTimeout: time.Second,
					WriteTimeout:      time.Second,
					IdleTimeout:       time.Second,
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

	// --- Start Daemons Sequentially ---
	s.logger.Info("Starting daemons sequentially...")
	var startupFailed bool
	for _, daemon := range s.daemons {
		s.logger.Info("Starting daemon", "daemon_name", daemon.Name())
		if err := daemon.Start(); err != nil {
			s.logger.Error("Failed to start daemon, initiating shutdown",
				"daemon_name", daemon.Name(),
				"error", err)
			// Send the specific error that caused the failure
			serverError <- fmt.Errorf("daemon %q failed to start: %w", daemon.Name(), err)
			startupFailed = true
			break // Stop starting other daemons if one fails
		}
		s.logger.Info("Daemon started successfully", "daemon_name", daemon.Name())
	}

	if !startupFailed {
		s.logger.Info("All daemons started successfully.")
	}
	// If startupFailed is true, an error was already sent to serverError

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
	shutdownTimeout := serverCfg.ShutdownGracefulTimeout.Duration
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

	// --- Stop Daemons Concurrently ---
	s.logger.Info("Stopping daemons...")
	for _, d := range s.daemons {
		daemon := d // Capture loop variable
		shutdownGroup.Go(func() error {
			s.logger.Info("Stopping daemon", "daemon_name", daemon.Name())
			err := daemon.Stop(gracefulCtx) // Pass the shutdown context
			if err != nil {
				// Log error but allow other daemons to attempt shutdown
				s.logger.Error("Error stopping daemon", "daemon_name", daemon.Name(), "error", err)
				// Return the error so errgroup knows about it
				return fmt.Errorf("daemon %q failed to stop gracefully: %w", daemon.Name(), err)
			}
			s.logger.Info("Daemon stopped gracefully", "daemon_name", daemon.Name())
			return nil
		})
	}

	// Wait for all shutdown tasks (HTTP servers + daemons)
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
		"readTimeout", cfg.ReadTimeout.Duration,
		"readHeaderTimeout", cfg.ReadHeaderTimeout.Duration,
		"writeTimeout", cfg.WriteTimeout.Duration,
		"idleTimeout", cfg.IdleTimeout.Duration)

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
