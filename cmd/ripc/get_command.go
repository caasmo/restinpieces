package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

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

func handleGetCommand(secureStore config.SecureStore, scopeName string, filter string) {
	if scopeName == "" {
		scopeName = config.ScopeApplication
	}
	decryptedData, _, err := secureStore.Get(scopeName, 0) // generation 0 = latest
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to retrieve/decrypt latest config for scope '%s': %v\n", scopeName, err)
		os.Exit(1)
	}

	tree, err := toml.LoadBytes(decryptedData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load TOML data for scope '%s'. Content may not be TOML or is corrupted: %v\n", scopeName, err)
		os.Exit(1)
	}

	allPathsWithValues := make(map[string]interface{})
	listTomlPathsWithValuesRecursive(tree, "", &allPathsWithValues)

	if len(allPathsWithValues) == 0 {
		fmt.Printf("No TOML paths with values found in configuration for scope '%s'.\n", scopeName)
		return
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
		fmt.Printf("No TOML paths with values matching '%s' found in scope '%s'.\n", filter, scopeName)
		return
	}

	sort.Strings(filteredPaths) // Ensure consistent order for output

	fmt.Printf("TOML paths with values for latest configuration in scope '%s':\n", scopeName)
	for _, path := range filteredPaths {
		value := allPathsWithValues[path]
		fmt.Printf("%s = %v\n", path, value)
	}
}
