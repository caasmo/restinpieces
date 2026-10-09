package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml"
)

var (
	// ErrReadInput is returned when the walk cannot read a line from the input.
	ErrReadInput = errors.New("failed to read input")
)

func printWalkUsage(w io.Writer) {
	help := Spec{
		Usage:       "walk [options] <filter>",
		Description: "Walks through configuration values and updates them one at a time.",
		Args: []ArgSpec{
			{"filter", "Substring filter on configuration paths; only matching values are walked"},
		},
		Options: []OptSpec{
			commandOptions.Opt("scope"),
		},
		Examples: []string{
			"ripc walk server",
			"ripc walk --scope my-app backup",
		},
	}
	help.Print(w, prog)
}

// handleWalkCommand parses the arguments for the 'walk' command and executes
// the core logic, returning any error to the caller.
func handleWalkCommand(secureStore config.SecureStore, in io.Reader, args []string, ui UI) error {
	opts, err := parseWalkArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printWalkUsage(ui.Out)
			return nil
		}
		printWalkUsage(ui.Err)
		return err
	}
	return walkConfig(ui, in, secureStore, opts.Scope, opts.Filter)
}

// walkConfig contains the testable core logic for the 'walk' command. It shows
// one matching value at a time and reads one input line per value.
func walkConfig(ui UI, in io.Reader, secureStore config.SecureStore, scope string, filter string) error {
	if scope == "" {
		scope = config.ScopeApplication
	}

	decryptedData, fileFormat, err := secureStore.Get(scope, 0) // generation 0 = latest
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve latest config for scope '%s': %w", ErrSecureStoreGet, scope, err)
	}

	parser, err := NewTomlParser(decryptedData, filter)
	if err != nil {
		return fmt.Errorf("%w: failed to load config data for scope '%s': %w", ErrConfigUnmarshal, scope, err)
	}

	tree, entries, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("%w: failed to load config data for scope '%s': %w", ErrConfigUnmarshal, scope, err)
	}

	if len(entries) == 0 {
		_, err = fmt.Fprintf(ui.Err, "No TOML paths with values matching '%s' found in scope '%s'.\n", filter, scope)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrWriteOutput, err)
		}
		return nil
	}

	_, err = fmt.Fprintf(ui.Err, "Matched %d values in scope '%s'\n", len(entries), scope)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	paths := make([]string, 0, len(entries))
	for path := range entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	scanner := bufio.NewScanner(in)
	changedCount := 0
	lastChangedPath := ""
	for _, path := range paths {
		entry := entries[path]

		err = printWalkEntry(ui, path, entry)
		if err != nil {
			return err
		}

		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		value, err := parseWalkValue(line)
		if err != nil {
			return err
		}

		SetWithComment(tree, path, value)
		changedCount++
		lastChangedPath = path
	}

	err = scanner.Err()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrReadInput, err)
	}

	if changedCount == 0 {
		_, err = fmt.Fprintln(ui.Err, "No changes.")
		if err != nil {
			return fmt.Errorf("%w: %w", ErrWriteOutput, err)
		}
		return nil
	}

	updatedTomlBytes, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal updated config: %w", ErrConfigMarshal, err)
	}

	err = validateTomlAsConfig(updatedTomlBytes, lastChangedPath)
	if err != nil {
		return err
	}

	err = secureStore.Save(scope, updatedTomlBytes, fileFormat, "Walk updates")
	if err != nil {
		return fmt.Errorf("%w: failed to save updated config for scope '%s': %w", ErrSecureStoreSave, scope, err)
	}

	_, err = fmt.Fprintf(ui.Err, "Updated %d values in scope '%s'\n", changedCount, scope)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}

// printWalkEntry writes one walk screen: the path with its comment next to it,
// then the stored value as the default for the prompt.
func printWalkEntry(ui UI, path string, entry TomlEntry) error {
	header := path
	if entry.Comment != "" {
		header += "  (" + entry.Comment + ")"
	}
	_, err := fmt.Fprintf(ui.Out, "%s\n", header)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	_, err = fmt.Fprintf(ui.Out, "New value [%v]: ", entry.Value)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}

// parseWalkValue turns one input line into a value the way set does it: an "@"
// argument reads the file contents, anything else is parsed as a TOML value
// with a quoted string as fallback.
func parseWalkValue(line string) (interface{}, error) {
	value, isFile, err := tomlValueFromFile(line)
	if err != nil {
		return nil, err
	}
	if isFile {
		return value, nil
	}
	return tomlValueFromText(line)
}

// WalkOptions holds the parsed options for the 'walk' command.
type WalkOptions struct {
	Scope  string // --scope
	Filter string // positional filter argument
}

// parseWalkArgs parses the arguments for the 'walk' command.
func parseWalkArgs(args []string) (WalkOptions, error) {
	walkCmd := flag.NewFlagSet("walk", flag.ContinueOnError)
	walkCmd.SetOutput(io.Discard)
	scopeOpt := commandOptions.Opt("scope")

	var opts WalkOptions
	walkCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	err := walkCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return WalkOptions{}, flag.ErrHelp
		}
		return WalkOptions{}, fmt.Errorf("parsing walk flags: %w: %v", ErrInvalidFlag, err)
	}
	if walkCmd.NArg() < 1 {
		return WalkOptions{}, fmt.Errorf("'walk' requires a filter argument: %w", ErrMissingArgument)
	}
	if walkCmd.NArg() > 1 {
		return WalkOptions{}, fmt.Errorf("'walk' command takes at most one filter argument: %w", ErrTooManyArguments)
	}
	opts.Filter = walkCmd.Arg(0)
	return opts, nil
}
