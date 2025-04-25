package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	// Keep db and dbz
	"github.com/caasmo/restinpieces"                  // Import restinpieces for pool creation
	"github.com/caasmo/restinpieces/config"           // Import config package
	dbz "github.com/caasmo/restinpieces/db/zombiezen" // Import zombiezen implementation
)


func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	outputFileFlag := flag.String("output", "", "Output file path (writes to stdout if empty)")
	flag.StringVar(outputFileFlag, "o", "", "Output file path (shorthand)") // Link shorthand to the same variable
	ageKeyPathFlag := flag.String("age-key", "", "Path to the age identity file (private key 'AGE-SECRET-KEY-1...') (required)")
	scopeFlag := flag.String("scope", config.ScopeApplication, "Scope for the configuration (e.g., 'application', 'plugin_x')")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -age-key <identity-file> <db-file> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Retrieves the latest configuration for a scope, decrypts it using an age identity, and writes it to output.\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  <db-file>          Path to the SQLite database file (required)\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *ageKeyPathFlag == "" {
		logger.Error("missing required flag: -age-key")
		flag.Usage()
		os.Exit(1)
	}
	if flag.NArg() != 1 {
		logger.Error("missing required argument: <db-file>")
		flag.Usage()
		os.Exit(1)
	}
	dbPath := flag.Arg(0)

	// --- Database Setup ---
	// Create the pool explicitly
	logger.Info("creating sqlite database pool", "path", dbPath)
	pool, err := restinpieces.NewZombiezenPool(dbPath)
	if err != nil {
		logger.Error("failed to create database pool", "db_path", dbPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		logger.Info("closing database pool")
		if err := pool.Close(); err != nil { // Close the pool
			logger.Error("error closing database pool", "error", err)
		}
	}()

	// Instantiate DB implementation using the pool
	dbImpl, err := dbz.New(pool) // Pass the pool to New
	if err != nil {
		logger.Error("failed to instantiate zombiezen db from pool", "error", err)
		os.Exit(1)
	}
	// Note: dbImpl.Close() is now a no-op or might not exist, pool closing is handled above.

	// --- Instantiate SecureConfig ---
	secureCfg, err := config.NewSecureConfigAge(dbImpl, *ageKeyPathFlag, logger)
	if err != nil {
		logger.Error("failed to instantiate secure config (age)", "age_key_path", *ageKeyPathFlag, "error", err)
		os.Exit(1)
	}

	logger.Info("retrieving latest configuration", "scope", *scopeFlag)
	decryptedData, err := secureCfg.Latest(*scopeFlag)
	if err != nil {
		logger.Error("failed to retrieve latest config via SecureConfig", "scope", *scopeFlag, "error", err)
		os.Exit(1)
	}

	if *outputFileFlag != "" {
		err := os.WriteFile(*outputFileFlag, decryptedData, 0644)
		if err != nil {
			logger.Error("failed to write config file",
				"path", *outputFileFlag,
				"error", err)
			os.Exit(1)
		}
		logger.Info("config written to file", "path", *outputFileFlag, "scope", *scopeFlag)
	} else {
		if _, err := os.Stdout.Write(decryptedData); err != nil {
			logger.Error("failed to write config to stdout", "scope", *scopeFlag, "error", err)
			os.Exit(1)
		}
		// Optionally log success to stderr if writing to stdout
		logger.Info("config written to stdout", "scope", *scopeFlag)
	}
}
