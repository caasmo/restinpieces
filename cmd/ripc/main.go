package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	dbz "github.com/caasmo/restinpieces/db/zombiezen"
)

var (
	// main application errors
	ErrMissingFlag        = errors.New("missing required global flag")
	ErrMissingCommand     = errors.New("missing command")
	ErrUnknownCommand     = errors.New("unknown command")
	ErrDBNotFound         = errors.New("database file not found")
	ErrDBAlreadyExists    = errors.New("database file already exists")
	ErrCreateDbPool       = errors.New("failed to create database pool")
	ErrCreateDbImpl       = errors.New("failed to instantiate zombiezen db from pool")
	ErrCreateSecureStore  = errors.New("failed to instantiate secure store")
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// discoverAgeKey checks for an age key, using a provided path or searching default locations.
func discoverAgeKey(providedKey string) (string, error) {
	if providedKey != "" {
		return providedKey, nil
	}
	defaultKeys := []string{"age_key.txt", "age.key"}
	for _, keyFile := range defaultKeys {
		if _, err := os.Stat(keyFile); err == nil {
			return keyFile, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("error checking for default key file %s: %w", keyFile, err)
		}
	}
	return "", fmt.Errorf("%w: -agekey flag must be provided, or 'age_key.txt' or 'age.key' must exist in the current directory", ErrMissingFlag)
}

// discoverDBPath checks for a database file, using a provided path or a default name.
func discoverDBPath(providedDB string) (string, error) {
	if providedDB != "" {
		return providedDB, nil
	}
	const defaultDB = "app.db"
	if _, err := os.Stat(defaultDB); err == nil {
		return defaultDB, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("error checking for default database file %s: %w", defaultDB, err)
	}
	return "", fmt.Errorf("%w: -dbpath flag must be provided, or 'app.db' must exist in the current directory", ErrMissingFlag)
}

func run(args []string, output io.Writer) error {
	// We need a new flag set for each run
	fs := flag.NewFlagSet("ripc", flag.ContinueOnError)
	fs.SetOutput(output)

	// Global flags
	ageIdentityPathFlag := fs.String("agekey", "", "Path to the age identity file (private key 'AGE-SECRET-KEY-1...')")
	dbPathFlag := fs.String("dbpath", "", "Path to the SQLite database file")

	fs.Usage = func() {
		help := CommandHelp{
			Usage:       "ripc [global options] <command> [command-specific options]",
			Description: "A tool for managing the Rip application, including configuration, authentication, and jobs.",
			GlobalOptions: map[string]Option{
				"agekey": {Usage: "Path to the age identity file (private key 'AGE-SECRET-KEY-1...')"},
				"dbpath": {Usage: "Path to the SQLite database file"},
			},
			Subcommands: []SubcommandGroup{
				{
					Subcommands: []Subcommand{
						{"app", "Manage application lifecycle (e.g., creating the database)"},
						{"config", "Manage the application's secure configuration"},
						{"auth", "Manage authentication settings (e.g., JWT secrets, OAuth2 providers)"},
						{"job", "Manage background jobs"},
						{"log", "Manage the log database"},
						{"help", "Show help for a specific command"},
					},
				},
			},
			Examples: []string{
				"ripc app create",
				"ripc config set server.port 8080",
				"ripc auth rotate-jwt-secrets",
			},
		}
		help.Print(output, "ripc")
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidFlag, err)
	}

	finalAgeKeyPath, err := discoverAgeKey(*ageIdentityPathFlag)
	if err != nil {
		fs.Usage()
		return err
	}
	*ageIdentityPathFlag = finalAgeKeyPath

	finalDBPath, err := discoverDBPath(*dbPathFlag)
	if err != nil {
		fs.Usage()
		return err
	}
	*dbPathFlag = finalDBPath

	cmdArgs := fs.Args()
	if len(cmdArgs) < 1 {
		fs.Usage()
		return nil // Successfully show usage and exit.
	}

	command := cmdArgs[0]
	commandArgs := cmdArgs[1:]

	isAppCreate := command == "app" && len(commandArgs) > 0 && commandArgs[0] == "create"
	if !isAppCreate {
		if _, err := os.Stat(*dbPathFlag); os.IsNotExist(err) {
			// Not using the writeUsage helper here as this is a specific error message, not part of the general usage.
			_, _ = fmt.Fprintf(output, "Error: database file not found: %s\n", *dbPathFlag)
			_, _ = fmt.Fprintf(output, "Please create it first using 'ripc app create'.\n")
			return ErrDBNotFound
		}
	} else { // for app create, the database must NOT exist
		if _, err := os.Stat(*dbPathFlag); err == nil {
			_, _ = fmt.Fprintf(output, "Error: database file already exists: %s\n", *dbPathFlag)
			return ErrDBAlreadyExists
		}
	}

	pool, err := restinpieces.NewZombiezenPool(*dbPathFlag)
	if err != nil {
		return fmt.Errorf("%w (db_path: %s): %v", ErrCreateDbPool, *dbPathFlag, err)
	}
	defer func() {
		if err := pool.Close(); err != nil {
			_, _ = fmt.Fprintf(output, "Error: error closing database pool: %v\n", err)
		}
	}()

	dbImpl, err := dbz.New(pool)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCreateDbImpl, err)
	}

	secureStore, err := config.NewSecureStoreAge(dbImpl, *ageIdentityPathFlag)
	if err != nil {
		return fmt.Errorf("%w (age, age_key_path: %s): %v", ErrCreateSecureStore, *ageIdentityPathFlag, err)
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
	case "log":
		handleLogCommand(secureStore, *dbPathFlag, commandArgs)
	case "help":
		handleHelpCommand(commandArgs, fs.Usage)
	default:
		fs.Usage()
		return fmt.Errorf("%w: %s", ErrUnknownCommand, command)
	}
	return nil
}
