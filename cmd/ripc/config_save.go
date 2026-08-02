package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/caasmo/restinpieces/config"
)

var (
	// ErrReadFileFailed is returned when the input file cannot be read.
	ErrReadFileFailed = errors.New("failed to read file")
)

// handleSaveCommand is the command-level wrapper. It executes the core logic
// and returns any error to the caller.
func handleSaveCommand(secureStore config.SecureStore, opts ConfigSaveOptions, ui UI) error {
	return saveConfigFromFile(ui.Out, secureStore, opts.Scope, opts.Format, opts.Desc, opts.Filename)
}

// saveConfigFromFile reads the specified file and passes its content to the core save logic.
func saveConfigFromFile(stdout io.Writer, secureStore config.SecureStore, scope, format, desc, filename string) error {
	fileData, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrReadFileFailed, filename, err)
	}
	return saveConfigFromData(stdout, secureStore, scope, filename, fileData, format, desc)
}

// saveConfigFromData contains the testable core logic for saving a config from a file.
// It accepts io.Writer for output, making it easy to test.
func saveConfigFromData(stdout io.Writer, secureStore config.SecureStore, scope, filename string, data []byte, format, desc string) error {
	resolvedFormat := format // Start with format from flag
	if resolvedFormat == "" {
		// No format flag, so derive from extension.
		extension := filepath.Ext(filename)
		if extension != "" {
			// Trim the leading dot.
			resolvedFormat = strings.TrimPrefix(extension, ".")
		}
	}

	if scope == "" {
		scope = config.ScopeApplication
	}

	description := desc
	if description == "" {
		description = fmt.Sprintf("Inserted from file: %s", filepath.Base(filename))
	}

	err := secureStore.Save(scope, data, resolvedFormat, description)
	if err != nil {
		return fmt.Errorf("%w: failed to save config to database for scope '%s': %w", ErrSecureStoreSave, scope, err)
	}

	if _, err := fmt.Fprintf(stdout, "Successfully saved file '%s' to scope '%s' in database\n", filename, scope); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// ConfigSaveOptions holds the parsed options for the 'config save' subcommand.
type ConfigSaveOptions struct {
	Scope    string // --scope
	Format   string // --format
	Desc     string // --desc
	Filename string // positional filename argument
}

// parseSaveArgs parses the arguments for the 'save' subcommand.
func parseSaveArgs(args []string) (ConfigSaveOptions, error) {
	saveCmd := flag.NewFlagSet("save", flag.ContinueOnError)
	saveCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]
	formatOpt := commandConfig.Options["format"]
	descOpt := commandConfig.Options["desc"]

	var opts ConfigSaveOptions
	saveCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	saveCmd.StringVar(&opts.Format, "format", "", formatOpt.Usage) // Corrected default value
	saveCmd.StringVar(&opts.Desc, "desc", descOpt.DefaultValue, descOpt.Usage)

	err := saveCmd.Parse(args)
	if err != nil {
		return ConfigSaveOptions{}, fmt.Errorf("parsing save flags: %w: %v", ErrInvalidFlag, err)
	}
	if saveCmd.NArg() < 1 {
		return ConfigSaveOptions{}, fmt.Errorf("'save' requires filename argument: %w", ErrMissingArgument)
	}
	if saveCmd.NArg() > 1 {
		return ConfigSaveOptions{}, fmt.Errorf("'save' command takes at most one filename argument: %w", ErrTooManyArguments)
	}
	opts.Filename = saveCmd.Arg(0)
	return opts, nil
}
