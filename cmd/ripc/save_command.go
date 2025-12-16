package main

import (
	"errors"
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
// and handles exiting the process on error.
func handleSaveCommand(secureStore config.SecureStore, scope, format, desc, filename string) {
	if err := saveConfigFromFile(os.Stdout, secureStore, scope, format, desc, filename); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
