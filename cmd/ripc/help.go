package main

// help.go is the shared, copy-paste help framework for the project's CLIs.
//
// Do not add imports beyond the standard library, so the file stays copyable
// into any CLI.
//
// The framework is fully data-driven: every command declares a Spec literal
// (Usage, Description, Args, Options, Subcommands, Examples) and one shared
// renderer, Spec.Print, produces the help text. Flag defaults and usage text
// used at registration time are read back from the same Spec via Spec.Opt.
//
// Inline example — a command with one positional argument and two options:
//
//	var corpusLsSpec = Spec{
//		Usage:       "corpus ls [options] [filter]",
//		Description: "List all documents in the corpus staging database.",
//		Args: []ArgSpec{
//			{"filter", "Optional substring filter on document labels"},
//		},
//		Options: []OptSpec{
//			{Name: "db", Meta: "FILE", Usage: "Corpus SQLite file (or SEGROB_CORPUS_DB)"},
//			{Name: "with-nlp", Aliases: []string{"w"}, Usage: "Only list records that have NLP data"},
//		},
//	}
//
//	// print the help: Spec.Print(w, prog, "corpus", "ls")
//
// Flag registration keeps the spec as the single source of truth:
//
//	fs.BoolVar(&opts.WithNlp, "with-nlp", false, corpusLsSpec.Opt("with-nlp").Usage)
import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ArgSpec documents one positional argument of a command.
type ArgSpec struct {
	Name  string // argument name shown in the Arguments section
	Usage string // one-line description
}

// OptSpec documents one command-line option.
//
// Name is the canonical flag name ("with-nlp"). Aliases are short forms
// ("w"), rendered as "-w, --with-nlp". Meta is the value placeholder for
// non-bool options ("FILE", "N", "INDEX"); leave it empty for bool flags.
// DefaultValue is printed in help and read back by Spec.Opt for flag
// registration. Usage is the one-line description.
type OptSpec struct {
	Name         string
	Aliases      []string
	Meta         string
	DefaultValue string
	Usage        string
}

// Subcommand is one entry of a command group's help.
type Subcommand struct {
	Name        string
	Description string
}

// SubcommandGroup clusters Subcommands under a Title (e.g. "Reading
// Configuration"). A zero Title renders the group without a heading.
type SubcommandGroup struct {
	Title       string
	Subcommands []Subcommand
}

// Spec is the declarative description of one command's (or command group's)
// help. Every section is optional; empty slices and strings are skipped.
type Spec struct {
	Usage         string
	Description   string
	Args          []ArgSpec
	Options       []OptSpec
	GlobalOptions []OptSpec
	Subcommands   []SubcommandGroup
	Examples      []string
}

// Opt returns the option registered under name, searching Options first and
// then GlobalOptions. It returns a zero OptSpec when name is unknown, so
// callers can register flags with empty default and usage.
func (s *Spec) Opt(name string) OptSpec {
	for _, o := range s.Options {
		if o.Name == name {
			return o
		}
	}
	for _, o := range s.GlobalOptions {
		if o.Name == name {
			return o
		}
	}
	return OptSpec{}
}

// display renders the flags column of one option: "-w, --with-nlp FILE".
func (o OptSpec) display() string {
	names := []string{"-" + o.Name}
	for _, a := range o.Aliases {
		names = append(names, "-"+a)
	}
	left := strings.Join(names, ", ")
	if o.Meta != "" {
		left += " " + o.Meta
	}
	return left
}

// Column widths for the two-column help sections.
const (
	helpCmdFmt = "  %-22s %s\n" // subcommands and arguments
	helpOptFmt = "  %-28s %s\n" // options
)

// Print renders the spec to w. prog is the invoked program name and
// parentCommands is the command path ("config", "set") used in the
// "For detailed help on a subcommand" footer; omit it for leaf help.
func (s *Spec) Print(w io.Writer, prog string, parentCommands ...string) {
	var err error

	firstSectionPrinted := false
	printSectionSeparator := func() {
		if firstSectionPrinted {
			if err != nil {
				return
			}
			_, err = fmt.Fprintln(w)
		}
		firstSectionPrinted = true
	}

	printOptions := func(title string, options []OptSpec) {
		if len(options) == 0 {
			return
		}
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(w, title)
		if err != nil {
			return
		}
		for _, o := range options {
			if err != nil {
				return
			}
			usage := o.Usage
			if o.DefaultValue != "" {
				usage = fmt.Sprintf("%s (default: %q)", usage, o.DefaultValue)
			}
			_, err = fmt.Fprintf(w, helpOptFmt, o.display(), usage)
			if err != nil {
				return
			}
		}
	}

	if s.Usage != "" {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(w, "Usage:")
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(w, "  %s %s\n", prog, s.Usage)
		if err != nil {
			return
		}
	}

	if s.Description != "" {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(w, "Description:")
		if err != nil {
			return
		}
		scanner := bufio.NewScanner(strings.NewReader(s.Description))
		for scanner.Scan() {
			if err != nil {
				return
			}
			_, err = fmt.Fprintf(w, "  %s\n", scanner.Text())
			if err != nil {
				return
			}
		}
	}

	if len(s.Args) > 0 {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(w, "Arguments:")
		if err != nil {
			return
		}
		for _, a := range s.Args {
			if err != nil {
				return
			}
			_, err = fmt.Fprintf(w, helpCmdFmt, a.Name, a.Usage)
			if err != nil {
				return
			}
		}
	}

	if len(s.Subcommands) > 0 {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(w, "Subcommands:")
		if err != nil {
			return
		}
		for _, group := range s.Subcommands {
			if group.Title != "" {
				if err != nil {
					return
				}
				_, err = fmt.Fprintln(w)
				if err != nil {
					return
				}
				_, err = fmt.Fprintf(w, "  %s:\n", group.Title)
				if err != nil {
					return
				}
			}
			for _, subcommand := range group.Subcommands {
				if err != nil {
					return
				}
				_, err = fmt.Fprintf(w, helpCmdFmt, subcommand.Name, subcommand.Description)
				if err != nil {
					return
				}
			}
		}
	}

	printOptions("Options:", s.Options)
	printOptions("Global Options:", s.GlobalOptions)

	if len(s.Examples) > 0 {
		printSectionSeparator()
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(w, "Examples:")
		if err != nil {
			return
		}
		for _, example := range s.Examples {
			if err != nil {
				return
			}
			_, err = fmt.Fprintf(w, "  %s\n", example)
			if err != nil {
				return
			}
		}
	}

	if len(parentCommands) > 0 {
		printSectionSeparator()
		if err != nil {
			return
		}
		cmdPath := strings.Join(parentCommands, " ")
		_, err = fmt.Fprintf(w, "For detailed help on a subcommand:\n  %s %s <subcommand> --help\n", prog, cmdPath)
		if err != nil {
			return
		}
	}
}
