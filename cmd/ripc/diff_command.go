package main

import (
	"errors"
	"fmt"
	"io"
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

var (
	ErrDiffGenerate    = errors.New("failed to generate diff")
)

func handleDiffCommand(secureStore config.SecureStore, scope string, generation int) {
	if scope == "" {
		scope = config.ScopeApplication
	}
	if err := diffConfig(os.Stdout, secureStore, scope, generation); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v ", err)
		os.Exit(1)
	}
}

// diffConfig contains the testable core logic for diffing configs.
// It accepts io.Writer for output, making it easy to test.
func diffConfig(stdout io.Writer, secureStore config.SecureStore, scope string, generation int) error {
	// Get latest config (generation 0)
	latestData, _, err := secureStore.Get(scope, 0)
	if err != nil {
		return fmt.Errorf("%w: failed to get latest config for scope '%s': %w", ErrSecureStoreGet, scope, err)
	}

	// Get target generation config
	targetData, _, err := secureStore.Get(scope, generation)
	if err != nil {
		return fmt.Errorf("%w: failed to get config generation %d for scope '%s': %w", ErrSecureStoreGet, generation, scope, err)
	}

	// Convert both to TOML strings for comparison
	var latestMap, targetMap map[string]interface{}
	if err := toml.Unmarshal(latestData, &latestMap); err != nil {
		return fmt.Errorf("%w: failed to parse latest config as TOML: %w", ErrConfigUnmarshal, err)
	}
	if err := toml.Unmarshal(targetData, &targetMap); err != nil {
		return fmt.Errorf("%w: failed to parse generation %d config as TOML: %w", ErrConfigUnmarshal, generation, err)
	}

	latestToml, err := toml.Marshal(latestMap)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal latest config: %w", ErrConfigMarshal, err)
	}

	targetToml, err := toml.Marshal(targetMap)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal generation %d config: %w", ErrConfigMarshal, generation, err)
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
		return fmt.Errorf("%w: %w", ErrDiffGenerate, err)
	}

	if strings.TrimSpace(result) == "" {
		if _, err := fmt.Fprintf(stdout, "No differences between generation %d and latest ", generation); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		return nil
	}

	if _, err := fmt.Fprintf(stdout, "Differences between generation %d and latest: ", generation); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	// Colorize the output
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		var err error
		switch {
		case strings.HasPrefix(line, "---"):
			_, err = fmt.Fprintf(stdout, "%s%s%s ", ColorBlue, line, ColorReset)
		case strings.HasPrefix(line, "+++"):
			_, err = fmt.Fprintf(stdout, "%s%s%s ", ColorBlue, line, ColorReset)
		case strings.HasPrefix(line, "@@"):
			_, err = fmt.Fprintf(stdout, "%s%s%s ", ColorCyan, line, ColorReset)
		case strings.HasPrefix(line, "-"):
			_, err = fmt.Fprintf(stdout, "%s%s%s ", ColorRed, line, ColorReset)
		case strings.HasPrefix(line, "+"):
			_, err = fmt.Fprintf(stdout, "%s%s%s ", ColorGreen, line, ColorReset)
		default:
			_, err = fmt.Fprintln(stdout, line)
		}
		if err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	return nil
}

