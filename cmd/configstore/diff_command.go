package main

import (
	"fmt"
	"os"
	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func handleDiffCommand(secureStore config.SecureStore, scope string, generation int) {
	if scope == "" {
		scope = config.ScopeApplication
	}

	// Get latest config (generation 0)
	latestData, _, err := secureStore.Get(scope, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get latest config: %v\n", err)
		os.Exit(1)
	}

	// Get target generation config
	targetData, _, err := secureStore.Get(scope, generation)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get config generation %d: %v\n", generation, err)
		os.Exit(1)
	}

	// Convert both to TOML strings for comparison
	var latestMap, targetMap map[string]interface{}
	if err := toml.Unmarshal(latestData, &latestMap); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse latest config as TOML: %v\n", err)
		os.Exit(1)
	}
	if err := toml.Unmarshal(targetData, &targetMap); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse generation %d config as TOML: %v\n", generation, err)
		os.Exit(1)
	}

	latestToml, err := toml.Marshal(latestMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal latest config: %v\n", err)
		os.Exit(1)
	}

	targetToml, err := toml.Marshal(targetMap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal generation %d config: %v\n", generation, err)
		os.Exit(1)
	}

	// Create diff instance
	dmp := diffmatchpatch.New()

	// Generate diff
	diffs := dmp.DiffMain(string(targetToml), string(latestToml), false)

	// Cleanup the diff for better readability
	diffs = dmp.DiffCleanupSemantic(diffs)
	diffs = dmp.DiffCleanupEfficiency(diffs)

	if len(diffs) == 1 && diffs[0].Type == diffmatchpatch.DiffEqual {
		fmt.Printf("No differences between generation %d and latest\n", generation)
		return
	}

	// Generate colored diff output
	fmt.Printf("Differences between generation %d (left) and latest (right):\n", generation)
	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			fmt.Printf("\x1b[32m%s\x1b[0m", diff.Text) // Green for additions
		case diffmatchpatch.DiffDelete:
			fmt.Printf("\x1b[31m%s\x1b[0m", diff.Text) // Red for deletions
		case diffmatchpatch.DiffEqual:
			fmt.Printf("%s", diff.Text) // No color for unchanged
		}
	}
	fmt.Println()
}
