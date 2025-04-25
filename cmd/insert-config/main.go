package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	// No longer need context, bytes, io, runtime, time, age, sqlite, sqlitex directly here
	// Keep db and dbz
	"github.com/caasmo/restinpieces"                  // Import restinpieces for pool creation
	"github.com/caasmo/restinpieces/config"           // Import config package
	"github.com/caasmo/restinpieces/db"               // Import db package for scope constant
	dbz "github.com/caasmo/restinpieces/db/zombiezen" // Import zombiezen implementation
)

// --- insertConfig function removed ---

func main() {
	// --- Setup Logger ---
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// --- Flag Parsing ---
	ageIdentityPathFlag := flag.String("age-key", "", "Path to the age identity file (private key 'AGE-SECRET-KEY-1...') (required)")
	dbPathFlag := flag.String("db", "", "Path to the SQLite database file (required)")
	filePathFlag := flag.String("file", "", "Path to the config file to insert (required)")
	scopeFlag := flag.String("scope", config.ScopeApplication, "Scope for the configuration (e.g., 'application', 'plugin_x')")
	formatFlag := flag.String("format", "toml", "Format of the configuration file (e.g., 'toml', 'json')")
	descFlag := flag.String("desc", "", "Optional description for this configuration version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -age-key <identity-file> -db <db-file> -file <config-file> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Encrypts a configuration file using an age identity and inserts it into the database.\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *ageIdentityPathFlag == "" || *dbPathFlag == "" || *filePathFlag == "" {
		flag.Usage()
		os.Exit(1)
	}

	// --- Database Setup ---
	// Create the pool explicitly
	logger.Info("creating sqlite database pool", "path", *dbPathFlag)
	pool, err := restinpieces.NewZombiezenPool(*dbPathFlag)
	if err != nil {
		logger.Error("failed to create database pool", "db_path", *dbPathFlag, "error", err)
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
	secureCfg, err := config.NewSecureConfigAge(dbImpl, *ageIdentityPathFlag, logger)
	if err != nil {
		logger.Error("failed to instantiate secure config (age)", "age_key_path", *ageIdentityPathFlag, "error", err)
		os.Exit(1)
	}

	// --- Read Input File ---
	configData, err := os.ReadFile(*filePathFlag)
	if err != nil {
		logger.Error("failed to read input config file", "path", *filePathFlag, "error", err)
		os.Exit(1)
	}

	// --- Determine Description ---
	description := *descFlag
	if description == "" {
		description = "Inserted from file: " + filepath.Base(*filePathFlag)
	}

	// --- Save Config using SecureConfig ---
	logger.Info("saving configuration", "scope", *scopeFlag, "format", *formatFlag, "file", *filePathFlag)
	err = secureCfg.Save(*scopeFlag, configData, *formatFlag, description)
	if err != nil {
		// SecureConfig.Save logs specifics, just log the failure here
		logger.Error("failed to save config via SecureConfig", "error", err)
		// fmt.Fprintf(os.Stderr, "Error: %v\n", err) // Keep stderr for script compatibility if needed
		os.Exit(1)
	}

	logger.Info("successfully inserted encrypted config",
		"scope", *scopeFlag,
		"format", *formatFlag,
		"file", *filePathFlag,
		"description", description)
}
