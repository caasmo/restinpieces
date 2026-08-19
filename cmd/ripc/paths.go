package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

// Custom errors for paths command
var (
	ErrTomlLoad = errors.New("failed to load TOML data")
)

func listTomlPathsRecursive(tree *toml.Tree, prefix string, paths *[]string) {
	currentPrefix := prefix
	if currentPrefix != "" {
		currentPrefix += "."
	}

	keys := tree.Keys()
	sort.Strings(keys) // Ensure consistent order

	for _, key := range keys {
		fullPath := currentPrefix + key
		value := tree.Get(key)
		if subTree, ok := value.(*toml.Tree); ok {
			listTomlPathsRecursive(subTree, fullPath, paths)
		} else {
			*paths = append(*paths, fullPath)
		}
	}
}

func printPathsUsage(w io.Writer) {
	help := Spec{
		Usage:       "paths [options] [filter]",
		Description: "Lists all keys in the configuration.",
		Args: []ArgSpec{
			{"filter", "Optional substring filter on configuration paths"},
		},
		Options: []OptSpec{
			commandOptions.Opt("scope"),
		},
		Examples: []string{
			"ripc paths",
			"ripc paths --scope my-app",
			"ripc paths server",
		},
	}
	help.Print(w, prog)
}

// handlePathsCommand parses the arguments for the 'paths' command and executes
// the core logic, returning any error to the caller.
func handlePathsCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parsePathsArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printPathsUsage(ui.Out)
			return nil
		}
		printPathsUsage(ui.Err)
		return err
	}
	return listPaths(ui, secureStore, opts.Scope, opts.Filter)
}

// listPaths contains the testable core logic for listing all paths in a TOML configuration.
// It accepts UI for output, making it easy to test.
func listPaths(ui UI, secureStore config.SecureStore, scopeName string, filter string) error {
	if scopeName == "" {
		scopeName = config.ScopeApplication
	}
	decryptedData, _, err := secureStore.Get(scopeName, 0) // generation 0 = latest
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve/decrypt latest config for scope '%s': %w", ErrSecureStoreGet, scopeName, err)
	}

	tree, err := toml.LoadBytes(decryptedData)
	if err != nil {
		return fmt.Errorf("%w: failed to load TOML data for scope '%s'. Content may not be TOML or is corrupted: %w", ErrTomlLoad, scopeName, err)
	}

	var allPaths []string
	listTomlPathsRecursive(tree, "", &allPaths)

	if len(allPaths) == 0 {
		if _, err := fmt.Fprintf(ui.Err, "No TOML paths found in configuration for scope '%s'.\n", scopeName); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		return nil
	}

	var filteredPaths []string
	if filter != "" {
		for _, p := range allPaths {
			if strings.Contains(p, filter) {
				filteredPaths = append(filteredPaths, p)
			}
		}
		allPaths = filteredPaths
	}

	if len(allPaths) == 0 {
		if _, err := fmt.Fprintf(ui.Err, "No TOML paths matching '%s' found in scope '%s'.\n", filter, scopeName); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		return nil
	}

	for _, p := range allPaths {
		if _, err := fmt.Fprintln(ui.Out, p); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}
	return nil
}

// PathsOptions holds the parsed options for the 'paths' command.
type PathsOptions struct {
	Scope  string // --scope
	Filter string // optional positional filter argument
}

// parsePathsArgs parses the arguments for the 'paths' command.
func parsePathsArgs(args []string) (PathsOptions, error) {
	pathsCmd := flag.NewFlagSet("paths", flag.ContinueOnError)
	pathsCmd.SetOutput(io.Discard)
	scopeOpt := commandOptions.Opt("scope")

	var opts PathsOptions
	pathsCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	err := pathsCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return PathsOptions{}, flag.ErrHelp
		}
		return PathsOptions{}, fmt.Errorf("parsing paths flags: %w: %v", ErrInvalidFlag, err)
	}
	if pathsCmd.NArg() > 1 {
		return PathsOptions{}, fmt.Errorf("'paths' command takes at most one filter argument: %w", ErrTooManyArguments)
	}
	if pathsCmd.NArg() > 0 {
		opts.Filter = pathsCmd.Arg(0)
	}
	return opts, nil
}
