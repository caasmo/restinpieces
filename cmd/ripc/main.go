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
	ErrMissingFlag         = errors.New("missing required global flag")
	ErrMissingCommand      = errors.New("missing command")
	ErrUnknownCommand      = errors.New("unknown command")
	ErrDBNotFound          = errors.New("database file not found")
	ErrDBAlreadyExists     = errors.New("database file already exists")
	ErrCreateDbPool        = errors.New("failed to create database pool")
	ErrCreateDbImpl        = errors.New("failed to instantiate zombiezen db from pool")
	ErrCreateSecureStore   = errors.New("failed to instantiate secure store")
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	// We need a new flag set for each run
	fs := flag.NewFlagSet("ripc", flag.ContinueOnError)
	fs.SetOutput(output)

	// Global flags
	ageIdentityPathFlag := fs.String("agekey", "", "Path to the age identity file (private key 'AGE-SECRET-KEY-1...')")
	dbPathFlag := fs.String("dbpath", "", "Path to the SQLite database file")

	var usageWriteErr error
	writeUsage := func(format string, a ...interface{}) {
		if usageWriteErr != nil {
			return
		}
		_, usageWriteErr = fmt.Fprintf(output, format, a...)
	}

	fs.Usage = func() {
		writeUsage("Usage: ripc [global options] <command> [command-specific options]\n\n")
		writeUsage("Manages securely stored configurations.\n\n")
		writeUsage("Global Options:\n")
		fs.PrintDefaults()
		writeUsage("\nAvailable Commands:\n")
		writeUsage("  app <subcommand> [options]       Manage application lifecycle (create)\n")
		writeUsage("  config <subcommand> [options]    Manage configuration (set, list, dump, etc.)\n")
		writeUsage("  auth <subcommand> [options]      Manage authentication (rotate-jwt-secrets, add-oauth2, etc.)\n")
		writeUsage("  job <subcommand> [options]       Manage background jobs (add, list, rm)\n")
		writeUsage("  log <subcommand> [options]       Manage the log database (init)\n")
	}

	if err := fs.Parse(args); err != nil {
		if usageWriteErr != nil {
			return usageWriteErr
		}
		return fmt.Errorf("%w: %v", ErrInvalidFlag, err)
	}

	if *ageIdentityPathFlag == "" {
		fs.Usage()
		if usageWriteErr != nil {
			return usageWriteErr
		}
		return fmt.Errorf("%w: -agekey", ErrMissingFlag)
	}
	if *dbPathFlag == "" {
		fs.Usage()
		if usageWriteErr != nil {
			return usageWriteErr
		}
		return fmt.Errorf("%w: -dbpath", ErrMissingFlag)
	}

	cmdArgs := fs.Args()
	if len(cmdArgs) < 1 {
		fs.Usage()
		if usageWriteErr != nil {
			return usageWriteErr
		}
		return ErrMissingCommand
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
		if usageWriteErr != nil {
			return usageWriteErr
		}
		return fmt.Errorf("%w: %s", ErrUnknownCommand, command)
	}
	return nil
}