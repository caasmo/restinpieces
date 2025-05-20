package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	dbz "github.com/caasmo/restinpieces/db/zombiezen"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

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

	dbImpl, err := dbz.New(pool) // Pass the pool to New
	if err != nil {
		logger.Error("failed to instantiate zombiezen db from pool", "error", err)
		os.Exit(1)
	}

	// --- Instantiate SecureConfig with early validation ---
	secureCfg, err := config.NewSecureStoreAge(dbImpl, *ageIdentityPathFlag)
	if err != nil {
		logger.Error("failed to initialize secure config store",
			"age_key_path", *ageIdentityPathFlag,
			"error", err)
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
		logger.Error("failed to save config via SecureConfig", "error", err)
		os.Exit(1)
	}

	logger.Info("successfully inserted encrypted config",
		"scope", *scopeFlag,
		"format", *formatFlag,
		"file", *filePathFlag,
		"description", description)
}
