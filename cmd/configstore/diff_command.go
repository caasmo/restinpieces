package main

import (
	"fmt"
	"os"
	"strings"
	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
	"github.com/pmezard/go-difflib/difflib"
)

// ANSI color codes
const (
	ColorReset = "\033[0m"
	ColorRed   = "\033[31m"
	ColorGreen = "\033[32m"
	ColorBlue  = "\033[34m"
	ColorCyan  = "\033[36m"
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

	// Generate unified diff using difflib
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(targetToml)),
		B:        difflib.SplitLines(string(latestToml)),
		FromFile: fmt.Sprintf("generation_%d", generation),
		ToFile:   "latest",
		Context:  1, // Set to 0 to show only changed lines
	}

	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to generate diff: %v\n", err)
		os.Exit(1)
	}

	if strings.TrimSpace(result) == "" {
		fmt.Printf("No differences between generation %d and latest\n", generation)
		return
	}

	fmt.Printf("Differences between generation %d and latest:\n\n", generation)
	
	// Colorize the output
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "---"):
			fmt.Printf("%s%s%s\n", ColorBlue, line, ColorReset)
		case strings.HasPrefix(line, "+++"):
			fmt.Printf("%s%s%s\n", ColorBlue, line, ColorReset)
		case strings.HasPrefix(line, "@@"):
			fmt.Printf("%s%s%s\n", ColorCyan, line, ColorReset)
		case strings.HasPrefix(line, "-"):
			fmt.Printf("%s%s%s\n", ColorRed, line, ColorReset)
		case strings.HasPrefix(line, "+"):
			fmt.Printf("%s%s%s\n", ColorGreen, line, ColorReset)
		default:
			fmt.Println(line)
		}
	}
}
