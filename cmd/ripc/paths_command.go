package main

import (
	"errors"
	"fmt"
	"io"
	"os"
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

// handlePathsCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handlePathsCommand(secureStore config.SecureStore, scopeName string, filter string) {
	if err := listPaths(os.Stdout, secureStore, scopeName, filter); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// listPaths contains the testable core logic for listing all paths in a TOML configuration.
// It accepts io.Writer for output, making it easy to test.
func listPaths(stdout io.Writer, secureStore config.SecureStore, scopeName string, filter string) error {
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
		if _, err := fmt.Fprintf(stdout, "No TOML paths found in configuration for scope '%s'.\n", scopeName); err != nil {
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
		if _, err := fmt.Fprintf(stdout, "No TOML paths matching '%s' found in scope '%s'.\n", filter, scopeName); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		return nil
	}

	if _, err := fmt.Fprintf(stdout, "Available TOML paths for latest configuration in scope '%s':\n", scopeName); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	for _, p := range allPaths {
		if _, err := fmt.Fprintln(stdout, p); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}
	return nil
}