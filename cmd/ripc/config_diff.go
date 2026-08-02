package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
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
	ErrDiffGenerate = errors.New("failed to generate diff")
)

func handleDiffCommand(secureStore config.SecureStore, opts ConfigDiffOptions, ui UI) error {
	if opts.Scope == "" {
		opts.Scope = config.ScopeApplication
	}
	return diffConfig(ui.Out, secureStore, opts.Scope, opts.Generation)
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
			_, err = fmt.Fprintf(stdout, "%s%s%s\n", ColorBlue, line, ColorReset)
		case strings.HasPrefix(line, "+++"):
			_, err = fmt.Fprintf(stdout, "%s%s%s\n", ColorBlue, line, ColorReset)
		case strings.HasPrefix(line, "@@"):
			_, err = fmt.Fprintf(stdout, "%s%s%s\n", ColorCyan, line, ColorReset)
		case strings.HasPrefix(line, "-"):
			_, err = fmt.Fprintf(stdout, "%s%s%s\n", ColorRed, line, ColorReset)
		case strings.HasPrefix(line, "+"):
			_, err = fmt.Fprintf(stdout, "%s%s%s\n", ColorGreen, line, ColorReset)
		default:
			_, err = fmt.Fprintln(stdout, line)
		}
		if err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	return nil
}

// ConfigDiffOptions holds the parsed options for the 'config diff' subcommand.
type ConfigDiffOptions struct {
	Scope      string // --scope
	Generation int    // positional generation argument
}

// parseDiffArgs parses the arguments for the 'diff' subcommand.
func parseDiffArgs(args []string) (ConfigDiffOptions, error) {
	diffCmd := flag.NewFlagSet("diff", flag.ContinueOnError)
	diffCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]

	var opts ConfigDiffOptions
	diffCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	err := diffCmd.Parse(args)
	if err != nil {
		return ConfigDiffOptions{}, fmt.Errorf("parsing diff flags: %w: %v", ErrInvalidFlag, err)
	}
	if diffCmd.NArg() < 1 {
		return ConfigDiffOptions{}, fmt.Errorf("'diff' requires generation number argument: %w", ErrMissingArgument)
	}
	if diffCmd.NArg() > 1 {
		return ConfigDiffOptions{}, fmt.Errorf("'diff' command takes at most one generation argument: %w", ErrTooManyArguments)
	}
	gen, err := strconv.Atoi(diffCmd.Arg(0))
	if err != nil {
		return ConfigDiffOptions{}, fmt.Errorf("generation must be a number: %w", ErrNotANumber)
	}
	opts.Generation = gen
	return opts, nil
}
