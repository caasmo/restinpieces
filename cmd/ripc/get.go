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

func printGetUsage(w io.Writer) {
	help := Spec{
		Usage:       "get [options] [filter]",
		Description: "Gets configuration values by path.",
		Args: []ArgSpec{
			{"filter", "Optional substring filter on configuration paths"},
		},
		Options: []OptSpec{
			commandOptions.Opt("scope"),
		},
		Examples: []string{
			"ripc get",
			"ripc get server",
			"ripc get --scope my-app server.port",
		},
	}
	help.Print(w, prog)
}

// handleGetCommand parses the arguments for the 'get' command and executes
// the core logic, returning any error to the caller.
func handleGetCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parseGetArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printGetUsage(ui.Out)
			return nil
		}
		printGetUsage(ui.Err)
		return err
	}
	return getAndPrintConfigPaths(ui, secureStore, opts.Scope, opts.Filter)
}

// getAndPrintConfigPaths contains the testable core logic for getting and printing config paths.
// It accepts UI for output, making it easy to test.
func getAndPrintConfigPaths(ui UI, secureStore config.SecureStore, scopeName string, filter string) error {
	if scopeName == "" {
		scopeName = config.ScopeApplication
	}
	decryptedData, _, err := secureStore.Get(scopeName, 0) // generation 0 = latest
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve/decrypt latest config for scope '%s': %w", ErrSecureStoreGet, scopeName, err)
	}

	tree, err := toml.LoadBytes(decryptedData)
	if err != nil {
		return fmt.Errorf("%w: failed to load TOML data for scope '%s': %w", ErrConfigUnmarshal, scopeName, err)
	}

	allPathsWithValues := make(map[string]interface{})
	listTomlPathsWithValuesRecursive(tree, "", &allPathsWithValues)

	if len(allPathsWithValues) == 0 {
		if _, err := fmt.Fprintf(ui.Err, "No TOML paths with values found in configuration for scope '%s'.\n", scopeName); err != nil {
			return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, err)
		}
		return nil
	}

	var filteredPaths []string
	if filter != "" {
		for path := range allPathsWithValues {
			if strings.Contains(path, filter) {
				filteredPaths = append(filteredPaths, path)
			}
		}
	} else {
		for path := range allPathsWithValues {
			filteredPaths = append(filteredPaths, path)
		}
	}

	if len(filteredPaths) == 0 {
		if _, err := fmt.Fprintf(ui.Err, "No TOML paths with values matching '%s' found in scope '%s'.\n", filter, scopeName); err != nil {
			return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, err)
		}
		return nil
	}

	sort.Strings(filteredPaths) // Ensure consistent order for output

	for _, path := range filteredPaths {
		value := allPathsWithValues[path]
		if _, err := fmt.Fprintf(ui.Out, "%s = %v\n", path, value); err != nil {
			return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, err)
		}
	}
	return nil
}

func listTomlPathsWithValuesRecursive(tree *toml.Tree, prefix string, pathsWithValues *map[string]interface{}) {
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
			listTomlPathsWithValuesRecursive(subTree, fullPath, pathsWithValues)
		} else {
			(*pathsWithValues)[fullPath] = value
		}
	}
}

// GetOptions holds the parsed options for the 'get' command.
type GetOptions struct {
	Scope  string // --scope
	Filter string // optional positional filter argument
}

// parseGetArgs parses the arguments for the 'get' command.
func parseGetArgs(args []string) (GetOptions, error) {
	getCmd := flag.NewFlagSet("get", flag.ContinueOnError)
	getCmd.SetOutput(io.Discard)
	scopeOpt := commandOptions.Opt("scope")

	var opts GetOptions
	getCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	err := getCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return GetOptions{}, flag.ErrHelp
		}
		return GetOptions{}, fmt.Errorf("parsing get flags: %w: %v", ErrInvalidFlag, err)
	}
	if getCmd.NArg() > 1 {
		return GetOptions{}, fmt.Errorf("'get' command takes at most one filter argument: %w", ErrTooManyArguments)
	}
	if getCmd.NArg() > 0 {
		opts.Filter = getCmd.Arg(0)
	}
	return opts, nil
}
