package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/core/proxy" // Import for BlockIp
	"github.com/caasmo/restinpieces/router"    // Import for NewChain
)

// Pool creation helpers moved to restinpieces package

func main() {
	// Define flags directly in main
	dbfile := flag.String("dbfile", "bench.db", "SQLite database file path")
	configFile := flag.String("config", "config.toml", "Path to configuration file")

	// Set custom usage message for the application
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Start the restinpieces application server.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	// Parse flags
	flag.Parse()

	// --- Create the Database Pool ---
	// Use the helper from the library to create a pool with suitable defaults.
	dbPool, err := restinpieces.NewCrawshawPool(*dbfile)
	// Or: dbPool, err := restinpieces.NewZombiezenPool(*dbfile)
	if err != nil {
		slog.Error("failed to create database pool", "error", err)
		os.Exit(1) // Exit if pool creation fails
	}
	// Defer closing the pool here, as main owns it now.
	// This must happen *after* the server finishes.
	defer func() {
		slog.Info("Closing database pool...")
		if err := dbPool.Close(); err != nil {
			slog.Error("Error closing database pool", "error", err)
		}
	}()

	// --- Initialize the Application ---
	app, srv, err := restinpieces.New(
		*configFile,
		restinpieces.WithDbCrawshaw(dbPool),
		restinpieces.WithRouterServeMux(),
		restinpieces.WithCacheRistretto(),
		restinpieces.WithTextLogger(nil),
	)
	if err != nil {
		slog.Error("failed to initialize application", "error", err)
		// Pool will be closed by the deferred function
		os.Exit(1) // Exit if app initialization fails
	}

	// --- User Customization of PreRouter ---
	// 'app' is now fully initialized with access to cache, logger, config, etc.

	// 1. Create Middleware Instances using App resources
	blockIpInstance := proxy.NewBlockIp(app.Cache(), app.Logger())
	// Add other middleware instances here if needed (e.g., metrics, custom logging)
	// myMetrics := metrics.New(app.Config().Metrics)

	// 2. Build the PreRouter Chain using router.Chain
	//    Start with app.Router() as the base handler for this chain.
	preRouterChain := router.NewChain(app.Router())

	// 3. Add Middleware (order matters: first added is outermost)
	//    Check config via app.Config() if middleware should be enabled.
	//    Example: if app.Config().BlockIp.Enabled { ... }
	if blockIpInstance.IsEnabled() { // Assuming IsEnabled checks internal state/config
		preRouterChain.WithMiddleware(blockIpInstance.Execute)
		slog.Info("IP Blocking middleware enabled in PreRouter chain")
	} else {
		slog.Info("IP Blocking middleware disabled")
	}
	// Add other middleware conditionally:
	// if myMetrics.IsEnabled() {
	//     preRouterChain.WithMiddleware(myMetrics.Middleware)
	// }
	// preRouterChain.WithMiddleware(mySimpleLoggerMiddleware) // Example simple logger

	// 4. Get the final composed handler for the pre-router steps
	finalPreRouterHandler := preRouterChain.Handler()

	// 5. Update the App's PreRouter
	app.SetPreRouter(finalPreRouterHandler)
	slog.Info("Custom PreRouter handler chain configured")
	// --- End Customization ---

	// Start the server
	// The Run method likely blocks until the server stops (e.g., via signal)
	// It will now use the handler set via app.SetPreRouter()
	srv.Run()

	slog.Info("Server shut down gracefully.")
	// No explicit os.Exit(0) needed, successful completion implies exit 0
}
