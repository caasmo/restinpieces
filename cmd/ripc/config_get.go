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

func printConfigGetUsage(w io.Writer) {
	help := Spec{
		Usage:       "config get [options] [filter]",
		Description: "Gets configuration values by path.",
		Args: []ArgSpec{
			{"filter", "Optional substring filter on configuration paths"},
		},
		Options: []OptSpec{
			commandConfig.Opt("scope"),
		},
		Examples: []string{
			"ripc config get",
			"ripc config get server",
			"ripc config get --scope my-app server.port",
		},
	}
	help.Print(w, prog, "config", "get")
}

// handleConfigGetCommand is the command-level wrapper. It executes the core logic
// and returns any error to the caller.
func handleConfigGetCommand(secureStore config.SecureStore, opts ConfigGetOptions, ui UI) error {
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

// ConfigGetOptions holds the parsed options for the 'config get' subcommand.
type ConfigGetOptions struct {
	Scope  string // --scope
	Filter string // optional positional filter argument
}

// parseConfigGetArgs parses the arguments for the 'get' subcommand.
func parseConfigGetArgs(args []string) (ConfigGetOptions, error) {
	getCmd := flag.NewFlagSet("get", flag.ContinueOnError)
	getCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Opt("scope")

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
