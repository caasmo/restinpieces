package main

import (
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
)

func printLogUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s log <subcommand> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Manages the logger database.\n\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  init    Initialize the log database and schema\n")
}

func handleLogCommand(secureStore config.SecureStore, dbPath string, commandArgs []string) {
	if len(commandArgs) < 1 {
		printLogUsage()
		os.Exit(1)
	}

	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "init":
		if len(subcommandArgs) > 0 {
			fmt.Fprintf(os.Stderr, "Error: 'init' does not take any arguments\n")
			printLogUsage()
			os.Exit(1)
		}
		handleLogInitCommand(secureStore, dbPath)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown log subcommand: %s\n", subcommand)
		printLogUsage()
		os.Exit(1)
	}
}
