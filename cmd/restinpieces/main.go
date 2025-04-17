package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces"
)

// Pool creation helpers moved to restinpieces package

func main() {
	// Define flags directly in main
	dbPath := flag.String("db", "", "Path to the SQLite database file (required)")
	ageKeyPath := flag.String("age-key", "", "Path to the age identity (private key) file (required)")

	// Set custom usage message for the application
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -db <database-path> -age-key <identity-file-path>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Start the restinpieces application server.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	// Parse flags
	flag.Parse()

	// Validate required flags
	if *dbPath == "" || *ageKeyPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	// --- Create the Database Pool ---
	// Use the helper from the library to create a pool with suitable defaults.
	//dbPool, err := restinpieces.NewCrawshawPool(*dbPath) // Use dbPath
	dbPool, err := restinpieces.NewZombiezenPool(*dbPath) // Use dbPath
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
	// Pass ageKeyPath as the first argument
	_, srv, err := restinpieces.New(
		*ageKeyPath, // Pass the age key file path
		restinpieces.WithDbZombiezen(dbPool),
		//restinpieces.WithDbCrawshaw(dbPool),
		restinpieces.WithRouterServeMux(),
		restinpieces.WithCacheRistretto(),
		restinpieces.WithTextLogger(nil),
	)
	if err != nil {
		slog.Error("failed to initialize application", "error", err)
		// Pool will be closed by the deferred function
		os.Exit(1) // Exit if app initialization fails
	}

	// PreRouter initialization is now handled internally within restinpieces.New / initPreRouter

	// Start the server
	// The Run method likely blocks until the server stops (e.g., via signal)
	// It will now use the handler set via app.SetPreRouter()
	srv.Run()

	slog.Info("Server shut down gracefully.")
	// No explicit os.Exit(0) needed, successful completion implies exit 0
}
