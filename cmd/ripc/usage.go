package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"strings"
)

// CommandHelp represents the information needed to generate help output.
type CommandHelp struct {
	Usage         string
	Description   string
	Subcommands   []SubcommandGroup
	Options       *flag.FlagSet
	GlobalOptions *flag.FlagSet
	Examples      []string
}

// SubcommandGroup allows clustering subcommands by meaning.
type SubcommandGroup struct {
	Title       string
	Subcommands []Subcommand
}

// Subcommand defines a subcommand for the help output.
type Subcommand struct {
	Name        string
	Description string
}

// Print formats and prints the help to the specified writer using a direct, imperative approach.
func (h *CommandHelp) Print(writer io.Writer, parentCommands ...string) {
	firstSectionPrinted := false

	// printSectionSeparator ensures there is exactly one blank line before each section.
	printSectionSeparator := func() {
		if firstSectionPrinted {
			fmt.Fprintln(writer)
		}
		firstSectionPrinted = true
	}

	// printFlags is a helper to print an indented flag set.
	printFlags := func(fs *flag.FlagSet) {
		var buf bytes.Buffer
		fs.SetOutput(&buf)
		fs.PrintDefaults()

		scanner := bufio.NewScanner(&buf)
		for scanner.Scan() {
			fmt.Fprintf(writer, "  %s\n", scanner.Text())
		}
	}

	if h.Usage != "" {
		printSectionSeparator()
		fmt.Fprintln(writer, "Usage:")
		fmt.Fprintf(writer, "  %s\n", h.Usage)
	}

	if h.Description != "" {
		printSectionSeparator()
		fmt.Fprintln(writer, "Description:")
		scanner := bufio.NewScanner(strings.NewReader(h.Description))
		for scanner.Scan() {
			fmt.Fprintf(writer, "  %s\n", scanner.Text())
		}
	}

	if len(h.Subcommands) > 0 {
		printSectionSeparator()
		fmt.Fprintln(writer, "Subcommands:")
		for _, group := range h.Subcommands {
			if group.Title != "" {
				// Add a newline before a new group title for separation
				fmt.Fprintln(writer)
				fmt.Fprintf(writer, "  %s:\n", group.Title)
			}
			for _, subcommand := range group.Subcommands {
				fmt.Fprintf(writer, "    %-22s %s\n", subcommand.Name, subcommand.Description)
			}
		}
	}

	if h.Options != nil {
		printSectionSeparator()
		fmt.Fprintln(writer, "Options:")
		printFlags(h.Options)
	}

	if h.GlobalOptions != nil {
		printSectionSeparator()
		fmt.Fprintln(writer, "Global Options:")
		printFlags(h.GlobalOptions)
	}

	if len(h.Examples) > 0 {
		printSectionSeparator()
		fmt.Fprintln(writer, "Examples:")
		for _, example := range h.Examples {
			fmt.Fprintf(writer, "  %s\n", example)
		}
	}

	// Footer
	if len(parentCommands) > 0 {
		printSectionSeparator()
		cmdPath := strings.Join(parentCommands, " ")
		fmt.Fprintf(writer, "For detailed help on a subcommand:\n  %s <subcommand> --help\n", cmdPath)
	}
}