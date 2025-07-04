package main

import (
	"fmt"
	"os"
)

func handleHelpCommand(args []string, mainUsage func()) {
	if len(args) == 0 {
		mainUsage()
		return
	}

	command := args[0]
	switch command {
	case "job":
		printJobUsage()
	case "config":
		printConfigUsage()
	case "auth":
		printAuthUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown help topic: %s\n", command)
		mainUsage()
		os.Exit(1)
	}
}
