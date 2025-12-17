package main

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Option defines a command-line option for documentation.
type Option struct {
	DefaultValue string
	Usage        string
}

// CommandHelp represents the information needed to generate help output.
type CommandHelp struct {
	Usage         string
	Description   string
	Subcommands   []SubcommandGroup
	Options       map[string]Option
	GlobalOptions map[string]Option
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

	printOptions := func(title string, options map[string]Option) {
		if len(options) == 0 {
			return
		}
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(writer, title)
		if err != nil {
			return
		}

		// Sort keys for consistent output order
		keys := make([]string, 0, len(options))
		for k := range options {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Mimic the formatting of flag.PrintDefaults
		for _, name := range keys {
			opt := options[name]
			if err != nil {
				return
			}
			// Assuming string type for all flags as per current implementation
			line := fmt.Sprintf("  -%s string", name)
			_, err = fmt.Fprintln(writer, line)
			if err != nil {
				return
			}
			usage := opt.Usage
			if opt.DefaultValue != "" {
				usage = fmt.Sprintf("%s (default: %q)", usage, opt.DefaultValue)
			}
			_, err = fmt.Fprintf(writer, "    	%s\n", usage)
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

	printOptions("Options:", h.Options)
	printOptions("Global Options:", h.GlobalOptions)

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