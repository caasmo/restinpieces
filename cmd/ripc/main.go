package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	dbz "github.com/caasmo/restinpieces/db/zombiezen"
)

func main() {
	// Global flags
	ageIdentityPathFlag := flag.String("age-key", "", "Path to the age identity file (private key 'AGE-SECRET-KEY-1...')")
	dbPathFlag := flag.String("dbpath", "", "Path to the SQLite database file")

	originalUsage := flag.Usage
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [global options] <command> [command-specific options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Manages securely stored configurations.\n\n")
		fmt.Fprintf(os.Stderr, "Global Options:\n")
		originalUsage() // Prints the global flags
		fmt.Fprintf(os.Stderr, "\nAvailable Commands:\n")
		fmt.Fprintf(os.Stderr, "  app <subcommand> [options]       Manage application lifecycle (create)\n")
		fmt.Fprintf(os.Stderr, "  config <subcommand> [options]    Manage configuration (set, list, dump, etc.)\n")
		fmt.Fprintf(os.Stderr, "  auth <subcommand> [options]      Manage authentication (rotate-jwt-secrets, add-oauth2, etc.)\n")
		fmt.Fprintf(os.Stderr, "  job <subcommand> [options]         Manage background jobs (add, list, rm)\n")
	}

	flag.Parse()

	if *ageIdentityPathFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: missing required global flag: -age-key\n")
		flag.Usage()
		os.Exit(1)
	}
	if *dbPathFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: missing required global flag: -dbpath\n")
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

	isAppCreate := command == "app" && len(commandArgs) > 0 && commandArgs[0] == "create"
	if !isAppCreate {
		if _, err := os.Stat(*dbPathFlag); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: database file not found: %s\n", *dbPathFlag)
			fmt.Fprintf(os.Stderr, "Please create it first using 'ripc app create'.\n")
			os.Exit(1)
		}
	} else { // for app create, the database must NOT exist
		if _, err := os.Stat(*dbPathFlag); err == nil {
			fmt.Fprintf(os.Stderr, "Error: database file already exists: %s\n", *dbPathFlag)
			os.Exit(1)
		}
	}

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
	case "app":
		handleAppCommand(secureStore, pool, *dbPathFlag, commandArgs)
	case "config":
		handleConfigCommand(secureStore, pool, commandArgs)
	case "auth":
		handleAuthCommand(secureStore, commandArgs)
	case "job":
		handleJobCommand(dbImpl, commandArgs)
	case "help":
		handleHelpCommand(commandArgs, flag.Usage)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

