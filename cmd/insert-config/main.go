package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	// No longer need context, bytes, io, runtime, time, age, sqlite, sqlitex directly here
	// Keep db and dbz
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
	scopeFlag := flag.String("scope", db.ConfigScopeApplication, "Scope for the configuration (e.g., 'application', 'plugin_x')")
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
	// Instantiate DB implementation (pool creation is handled inside New)
	dbImpl, err := dbz.New(*dbPathFlag)
	if err != nil {
		logger.Error("failed to instantiate zombiezen db", "db_path", *dbPathFlag, "error", err)
		os.Exit(1)
	}
	defer func() {
		logger.Info("closing database connection")
		if err := dbImpl.Close(); err != nil { // Use the Close method from the implementation
			logger.Error("error closing database connection", "error", err)
		}
	}()

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
