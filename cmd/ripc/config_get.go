package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

// handleGetCommand is the command-level wrapper. It executes the core logic
// and returns any error to the caller.
func handleGetCommand(secureStore config.SecureStore, opts ConfigGetOptions, ui UI) error {
	return getAndPrintConfigPaths(ui.Out, secureStore, opts.Scope, opts.Filter)
}

// getAndPrintConfigPaths contains the testable core logic for getting and printing config paths.
// It accepts io.Writer for output, making it easy to test.
func getAndPrintConfigPaths(stdout io.Writer, secureStore config.SecureStore, scopeName string, filter string) error {
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
		if _, err := fmt.Fprintf(stdout, "No TOML paths with values found in configuration for scope '%s'.\n", scopeName); err != nil {
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
		if _, err := fmt.Fprintf(stdout, "No TOML paths with values matching '%s' found in scope '%s'.\n", filter, scopeName); err != nil {
			return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, err)
		}
		return nil
	}

	sort.Strings(filteredPaths) // Ensure consistent order for output

	if _, err := fmt.Fprintf(stdout, "TOML paths with values for latest configuration in scope '%s':\n", scopeName); err != nil {
		return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, err)
	}
	for _, path := range filteredPaths {
		value := allPathsWithValues[path]
		if _, err := fmt.Fprintf(stdout, "%s = %v\n", path, value); err != nil {
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

// ConfigGetOptions holds the parsed options for the 'config get' subcommand.
type ConfigGetOptions struct {
	Scope  string // --scope
	Filter string // optional positional filter argument
}

// parseGetArgs parses the arguments for the 'get' subcommand.
func parseGetArgs(args []string) (ConfigGetOptions, error) {
	getCmd := flag.NewFlagSet("get", flag.ContinueOnError)
	getCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]

	var opts ConfigGetOptions
	getCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	err := getCmd.Parse(args)
	if err != nil {
		return ConfigGetOptions{}, fmt.Errorf("parsing get flags: %w: %v", ErrInvalidFlag, err)
	}
	if getCmd.NArg() > 1 {
		return ConfigGetOptions{}, fmt.Errorf("'get' command takes at most one filter argument: %w", ErrTooManyArguments)
	}
	if getCmd.NArg() > 0 {
		opts.Filter = getCmd.Arg(0)
	}
	return opts, nil
}
