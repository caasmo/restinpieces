package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	dbz "github.com/caasmo/restinpieces/db/zombiezen"
	// toml "github.com/pelletier/go-toml" // No longer needed directly in main
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Global flags
	ageIdentityPathFlag := flag.String("age-key", "", "Path to the age identity file (private key 'AGE-SECRET-KEY-1...')")
	dbPathFlag := flag.String("db", "", "Path to the SQLite database file")
	scopeFlag := flag.String("scope", config.ScopeApplication, "Scope for the configuration (e.g., 'application', 'plugin_x')")
	formatFlag := flag.String("format", "toml", "Format of the configuration file (e.g., 'toml', 'json')")
	descFlag := flag.String("desc", "", "Optional description for this configuration version")

	originalUsage := flag.Usage
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [global options] <command> [command-specific options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Manages securely stored configurations.\n\n")
		fmt.Fprintf(os.Stderr, "Global Options:\n")
		originalUsage() // Prints the global flags
		fmt.Fprintf(os.Stderr, "\nAvailable Commands:\n")
		fmt.Fprintf(os.Stderr, "  set <path> <value>   Set a configuration value. Prefix value with '@' to load from file.\n")
		// Add other commands here in the future
	}

	flag.Parse()

	if *ageIdentityPathFlag == "" {
		logger.Error("missing required global flag: -age-key")
		flag.Usage()
		os.Exit(1)
	}
	if *dbPathFlag == "" {
		logger.Error("missing required global flag: -db")
		flag.Usage()
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) < 1 {
		logger.Error("missing command")
		flag.Usage()
		os.Exit(1)
	}

	command := args[0]
	commandArgs := args[1:]

	logger.Info("creating sqlite database pool", "path", *dbPathFlag)
	pool, err := restinpieces.NewZombiezenPool(*dbPathFlag)
	if err != nil {
		logger.Error("failed to create database pool", "db_path", *dbPathFlag, "error", err)
		os.Exit(1)
	}
	defer func() {
		logger.Info("closing database pool")
		if err := pool.Close(); err != nil {
			logger.Error("error closing database pool", "error", err)
		}
	}()

	dbImpl, err := dbz.New(pool)
	if err != nil {
		logger.Error("failed to instantiate zombiezen db from pool", "error", err)
		os.Exit(1)
	}

	// Note: The user's file uses NewSecureConfigAge, which expects a logger.
	// If this was meant to be NewSecureStoreAge (which doesn't take a logger),
	// this line would need adjustment. Sticking to the provided file's usage.
	secureStore, err := config.NewSecureConfigAge(dbImpl, *ageIdentityPathFlag, logger)
	if err != nil {
		logger.Error("failed to instantiate secure store (age)", "age_key_path", *ageIdentityPathFlag, "error", err)
		os.Exit(1)
	}

	switch command {
	case "set":
		handleSetCommand(logger, secureStore, *scopeFlag, *formatFlag, *descFlag, commandArgs)
	default:
		logger.Error("unknown command", "command", command)
		flag.Usage()
		os.Exit(1)
	}
}
