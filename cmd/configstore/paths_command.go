package main

import (
	"fmt"
	"os"
	"sort"
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

func handlePathsCommand(secureStore config.SecureStore, scopeName string) {
	decryptedData, err := secureStore.Latest(scopeName)
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
	} else {
		fmt.Printf("Available TOML paths for latest configuration in scope '%s':\n", scopeName)
		for _, p := range allPaths {
			fmt.Println(p)
		}
	}
}
