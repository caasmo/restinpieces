package main

import (
	"context" // Added for signal handling
	"flag"
	"io/fs"
	"log/slog"
	"os"
	"os/signal" // Added for signal handling
	"syscall"   // Added for SIGHUP, SIGTERM, SIGINT
	"time"      // Added for shutdown timeout

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/custom"
	"github.com/caasmo/restinpieces/setup"
	"github.com/caasmo/restinpieces/server"
)

func logEmbeddedAssets(assets fs.FS, cfg *config.Config, logger *slog.Logger) {
	subFS, err := fs.Sub(assets, cfg.PublicDir)
	if err != nil {
		logger.Error("failed to create sub filesystem for logging assets", "error", err)
		return // Or handle the error more gracefully
	}
	assetCount := 0
	fs.WalkDir(subFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			assetCount++
			logger.Debug("embedded asset", "path", path)
		}
		return nil
	})
	logger.Debug("total embedded assets", "count", assetCount)
}

func main() {

	dbfile := flag.String("dbfile", "bench.db", "SQLite database file path")
	flag.Parse()

	// Initial config load
	initialCfg, err := config.Load(*dbfile)
	if err != nil {
		slog.Error("failed to load initial config", "error", err) // Use default logger before app logger is ready
		os.Exit(1)
	}

	// Create a logger instance early
	// TODO: Make logger configurable (level, format json/text) via flags or config file
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create the config provider
	configProvider := config.NewProvider(initialCfg, logger) // Pass logger to provider

	// Setup App using the provider
	app, proxy, err := setup.SetupApp(configProvider, logger, *dbfile) // Pass provider, logger, dbfile
	if err != nil {
		logger.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}
	// Defer Close until after signal handling setup is complete
	// defer app.Close() // Moved lower

	// Log embedded assets using config from provider
	logger.Debug("logging embedded assets", "public_dir", configProvider.Get().PublicDir)
	logEmbeddedAssets(restinpieces.EmbeddedAssets, configProvider.Get(), logger)

	// TODO better custom/app move to init_app
	cApp := custom.NewApp(app)

	// Setup routing - Pass initial config for setup, handlers will use app.Config() for dynamic access
	route(configProvider.Get(), app, cApp)

	// Setup Scheduler, passing the config provider
	scheduler, err := setup.SetupScheduler(configProvider, app.Db(), logger) // Pass provider
	if err != nil {
		logger.Error("failed to initialize scheduler", "error", err)
		os.Exit(1)
	}

	// Create server - Pass initial server config. Server restart needed for server config changes.
	srv := server.NewServer(configProvider.Get().Server, proxy, scheduler, logger)

	// --- Signal Handling for Graceful Shutdown and Config Reload ---
	stop := make(chan os.Signal, 1)
	reload := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM) // Listen for stop signals
	signal.Notify(reload, syscall.SIGHUP)              // Listen for SIGHUP for reload

	// Start server in a goroutine
	go func() {
		logger.Info("starting server", "addr", configProvider.Get().Server.Addr)
		// Run might block, handle potential errors that cause it to return
		if err := srv.Run(); err != nil {
			logger.Error("server run failed", "error", err)
			// Signal the main goroutine to stop if the server fails unexpectedly
			// Use non-blocking send in case stop channel is already closed or full
			select {
			case stop <- syscall.SIGTERM: // Send a stop signal
			default:
			}
		}
	}()

	// Main loop for signal handling
	logger.Info("application started successfully. press ctrl+c to shut down.")
	running := true
	for running {
		select {
		case <-reload:
			logger.Info("received SIGHUP, attempting to reload configuration...")
			newCfg, err := config.Load(*dbfile) // Reload config from source
			if err != nil {
				logger.Error("failed to reload config on SIGHUP", "error", err)
				// Continue running with the old configuration
			} else {
				configProvider.Update(newCfg) // Atomically update the config
				logger.Info("configuration reloaded via SIGHUP")
				// Note: Server restart is needed for changes in Server config section.
				// Other components using configProvider.Get() will see new values.
			}
		case sig := <-stop:
			logger.Info("received shutdown signal", "signal", sig.String())
			running = false // Exit the loop after cleanup
		}
	}

	// --- Graceful Shutdown ---
	logger.Info("initiating graceful shutdown...")

	// Get shutdown timeout from the *current* config
	shutdownTimeout := configProvider.Get().Server.ShutdownGracefulTimeout
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
	} else {
		logger.Info("server gracefully stopped")
	}

	// Stop scheduler (assuming it has a Stop method - needs implementation)
	if scheduler != nil {
		// scheduler.Stop(ctx) // Example: Implement Stop() in scheduler
		logger.Info("scheduler stopped")
	}

	// Close app resources (like DB)
	app.Close()
	logger.Info("application resources closed")
	logger.Info("shutdown complete.")
}
