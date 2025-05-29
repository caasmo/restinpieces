package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	// "strconv" // No longer needed

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
	// "zombiezen.com/go/sqlite/sqlitex" // No longer needed
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

func handlePathsCommand(secureStore config.SecureStore, scopeName string, filter string) {
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

	var allPaths []string
	listTomlPathsRecursive(tree, "", &allPaths)

	if len(allPaths) == 0 {
		fmt.Printf("No TOML paths found in configuration for scope '%s'.\n", scopeName)
		return
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
		fmt.Printf("No TOML paths matching '%s' found in scope '%s'.\n", filter, scopeName)
		return
	}

	fmt.Printf("Available TOML paths for latest configuration in scope '%s':\n", scopeName)
	for _, p := range allPaths {
		fmt.Println(p)
	}
}
