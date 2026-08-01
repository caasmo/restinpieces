package main

import (
	"errors"
	"fmt"
	"os"
)

// ErrUnknownHelpTopic is returned when a help topic is not found.
var ErrUnknownHelpTopic = errors.New("unknown help topic")

// Stored as variables to allow for easy mocking in tests.
var (
	printAppUsageFunc    = printAppUsage
	printJobUsageFunc    = printJobUsage
	printConfigUsageFunc = printConfigUsage
	printLogUsageFunc    = printLogUsage
)

// handleHelpCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleHelpCommand(args []string, mainUsage func(), ui UI) {
	if len(args) == 0 {
		mainUsage()
		return
	}

	topic := args[0]
	err := runHelpTopic(topic, ui)
	if err != nil {
		// We only expect ErrUnknownHelpTopic here.
		_, _ = fmt.Fprintf(ui.Err, "Error: unknown help topic: %s\n\n", topic)
		mainUsage()
		os.Exit(1)
	}
}

// runHelpTopic contains the testable core logic for dispatching to the
// correct help printer. It returns an error if the topic is not recognized.
func runHelpTopic(topic string, ui UI) error {
	switch topic {
	case "app":
		printAppUsageFunc(ui.Out)
	case "job":
		printJobUsageFunc(ui.Out)
	case "config":
		printConfigUsageFunc(ui.Out)
	case "log":
		printLogUsageFunc(ui.Out)
	default:
		return ErrUnknownHelpTopic
	}
	return nil
}
