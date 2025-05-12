package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	dbz "github.com/caasmo/restinpieces/db/zombiezen"
	// toml "github.com/pelletier/go-toml" // No longer needed directly in main
)

func main() {
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
		fmt.Fprintf(os.Stderr, "  scopes               List all unique configuration scopes found in the database.\n")
		fmt.Fprintf(os.Stderr, "  list [scope]         List configuration versions. If scope is omitted, lists for all scopes.\n")
		fmt.Fprintf(os.Stderr, "  paths <id>           List all TOML paths for a given configuration ID (if format is toml).\n")
		// Add other commands here in the future
	}

	flag.Parse()

	if *ageIdentityPathFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: missing required global flag: -age-key\n")
		flag.Usage()
		os.Exit(1)
	}
	if *dbPathFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: missing required global flag: -db\n")
		flag.Usage()
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: missing command\n")
		flag.Usage()
		os.Exit(1)
	}

	command := args[0]
	commandArgs := args[1:]

	pool, err := restinpieces.NewZombiezenPool(*dbPathFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create database pool (db_path: %s): %v\n", *dbPathFlag, err)
		os.Exit(1)
	}
	defer func() {
		if err := pool.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: error closing database pool: %v\n", err)
		}
	}()

	dbImpl, err := dbz.New(pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to instantiate zombiezen db from pool: %v\n", err)
		os.Exit(1)
	}

	secureStore, err := config.NewSecureStoreAge(dbImpl, *ageIdentityPathFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to instantiate secure store (age, age_key_path: %s): %v\n", *ageIdentityPathFlag, err)
		os.Exit(1)
	}

	switch command {
	case "set":
		handleSetCommand(secureStore, *scopeFlag, *formatFlag, *descFlag, commandArgs)
	case "scopes":
		if len(commandArgs) > 0 {
			fmt.Fprintf(os.Stderr, "Error: 'scopes' command does not take any arguments\n")
			flag.Usage()
			os.Exit(1)
		}
		handleScopesCommand(pool)
	case "list":
		scopeToList := ""
		if len(commandArgs) > 0 {
			scopeToList = commandArgs[0]
			if len(commandArgs) > 1 {
				fmt.Fprintf(os.Stderr, "Error: 'list' command takes at most one scope argument\n")
				flag.Usage()
				os.Exit(1)
			}
		}
		handleListCommand(pool, scopeToList)
	case "paths":
		if len(commandArgs) < 1 {
			fmt.Fprintf(os.Stderr, "Error: 'paths' command requires a configuration ID argument\n")
			flag.Usage()
			os.Exit(1)
		}
		if len(commandArgs) > 1 {
			fmt.Fprintf(os.Stderr, "Error: 'paths' command takes only one ID argument\n")
			flag.Usage()
			os.Exit(1)
		}
		configIDStr := commandArgs[0]
		handlePathsCommand(pool, secureStore, configIDStr)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}
