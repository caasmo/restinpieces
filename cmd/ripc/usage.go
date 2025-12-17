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
	var err error

	firstSectionPrinted := false
	printSectionSeparator := func() {
		if firstSectionPrinted {
			if err != nil {
				return
			}
			_, err = fmt.Fprintln(writer)
		}
		firstSectionPrinted = true
	}

	printFlags := func(fs *flag.FlagSet) {
		var buf bytes.Buffer
		fs.SetOutput(&buf)
		fs.PrintDefaults()

		scanner := bufio.NewScanner(&buf)
		for scanner.Scan() {
			if err != nil {
				return
			}
			_, err = fmt.Fprintf(writer, "  %s\n", scanner.Text())
		}
	}

	if h.Usage != "" {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(writer, "Usage:")
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(writer, "  %s\n", h.Usage)
	}

	if h.Description != "" {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(writer, "Description:")
		if err != nil {
			return
		}
		scanner := bufio.NewScanner(strings.NewReader(h.Description))
		for scanner.Scan() {
			if err != nil {
				return
			}
			_, err = fmt.Fprintf(writer, "  %s\n", scanner.Text())
		}
	}

	if len(h.Subcommands) > 0 {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(writer, "Subcommands:")
		if err != nil {
			return
		}
		for _, group := range h.Subcommands {
			if group.Title != "" {
				if err != nil {
					return
				}
				_, err = fmt.Fprintln(writer)
				if err != nil {
					return
				}
				_, err = fmt.Fprintf(writer, "  %s:\n", group.Title)
			}
			if err != nil {
				return
			}
			for _, subcommand := range group.Subcommands {
				if err != nil {
					return
				}
				_, err = fmt.Fprintf(writer, "    %-22s %s\n", subcommand.Name, subcommand.Description)
			}
		}
	}

	if h.Options != nil {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(writer, "Options:")
		if err != nil {
			return
		}
		printFlags(h.Options)
	}

	if h.GlobalOptions != nil {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(writer, "Global Options:")
		if err != nil {
			return
		}
		printFlags(h.GlobalOptions)
	}

	if len(h.Examples) > 0 {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(writer, "Examples:")
		if err != nil {
			return
		}
		for _, example := range h.Examples {
			if err != nil {
				return
			}
			_, err = fmt.Fprintf(writer, "  %s\n", example)
		}
	}

	if len(parentCommands) > 0 {
		printSectionSeparator()
		if err != nil {
			return
		}
		cmdPath := strings.Join(parentCommands, " ")
		_, err = fmt.Fprintf(writer, "For detailed help on a subcommand:\n  %s <subcommand> --help\n", cmdPath)
	}
}