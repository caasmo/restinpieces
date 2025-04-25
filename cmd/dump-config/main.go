package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	// Keep db and dbz
	"github.com/caasmo/restinpieces/config"           // Import config package
	dbz "github.com/caasmo/restinpieces/db/zombiezen" // Import zombiezen implementation
)

// --- ConfigDumper struct and methods removed ---

func main() {
	// --- Setup Logger ---
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// --- Flag Parsing ---
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

	// --- Validate Arguments and Flags ---
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
	dbImpl, err := dbz.New(dbPath)
	if err != nil {
		logger.Error("failed to instantiate zombiezen db", "db_path", dbPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		logger.Info("closing database connection")
		if err := dbImpl.Close(); err != nil {
			logger.Error("error closing database connection", "error", err)
		}
	}()

	// --- Instantiate SecureConfig ---
	secureCfg, err := config.NewSecureConfigAge(dbImpl, *ageKeyPathFlag, logger)
	if err != nil {
		logger.Error("failed to instantiate secure config (age)", "age_key_path", *ageKeyPathFlag, "error", err)
		os.Exit(1)
	}

	// --- Get Latest Config using SecureConfig ---
	logger.Info("retrieving latest configuration", "scope", *scopeFlag)
	decryptedData, err := secureCfg.Latest(*scopeFlag)
	if err != nil {
		// SecureConfig.Latest logs specifics, just log the failure here
		logger.Error("failed to retrieve latest config via SecureConfig", "scope", *scopeFlag, "error", err)
		// fmt.Fprintf(os.Stderr, "Error: %v\n", err) // Keep stderr for script compatibility if needed
		os.Exit(1)
	}

	// --- Write Output ---
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
		// Write to stdout
		if _, err := os.Stdout.Write(decryptedData); err != nil {
			logger.Error("failed to write config to stdout", "scope", *scopeFlag, "error", err)
			os.Exit(1)
		}
		// Optionally log success to stderr if writing to stdout
		logger.Info("config written to stdout", "scope", *scopeFlag)
	}
}
