package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
	"zombiezen.com/go/sqlite/sqlitex"
)

func printAppUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s app <subcommand> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Manages the application lifecycle.\n\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  create                Create a new application instance\n")
}

func handleAppCommand(secureStore config.SecureStore, dbPool *sqlitex.Pool, dbPath string, commandArgs []string) {
	if len(commandArgs) < 1 {
		printAppUsage()
		os.Exit(1)
	}

	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "create":
		createCmd := flag.NewFlagSet("create", flag.ExitOnError)
		createCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: %s app create\n\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "Creates a new application instance (database and initial config).\n")
			fmt.Fprintf(os.Stderr, "Relies on the global -dbpath and -age-key flags.\n")
		}
		if err := createCmd.Parse(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing create flags: %v\n", err)
			createCmd.Usage()
			os.Exit(1)
		}
		if createCmd.NArg() > 0 {
			fmt.Fprintf(os.Stderr, "Error: 'create' does not take any arguments\n")
			createCmd.Usage()
			os.Exit(1)
		}
		handleAppCreateCommand(secureStore, dbPool, dbPath)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown app subcommand: %s\n", subcommand)
		printAppUsage()
		os.Exit(1)
	}
}
