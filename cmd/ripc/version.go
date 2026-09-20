package main

import (
	"fmt"
	"io"
)

func printVersionUsage(w io.Writer) {
	help := Spec{
		Usage:       "version",
		Description: "Prints the ripc build tag and commit.",
		Examples: []string{
			"ripc version",
		},
	}
	help.Print(w, prog)
}

// handleVersionCommand prints the build tag and commit set at compile time
// via -ldflags "-X main.BuildTag=$VERSION -X main.BuildCommit=$COMMIT".
func handleVersionCommand(args []string, ui UI) error {
	if len(args) > 0 {
		return fmt.Errorf("'version' command does not take any arguments: %w", ErrTooManyArguments)
	}
	_, err := fmt.Fprintf(ui.Out, "%s %s\n", BuildTag, BuildCommit)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}
