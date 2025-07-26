package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml"
)

// Error definitions for set command
var (
	ErrMissingSetArguments = errors.New("missing path or value for 'set' command")
	ErrPathNotFound        = errors.New("configuration path does not exist")
	ErrReadFile            = errors.New("failed to read value from file")
	ErrParseValue          = errors.New("failed to parse value")
)

// handleSetCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleSetCommand(
	secureCfg config.SecureStore,
	scope string,
	format string,
	description string,
	cmdArgs []string) {

	if len(cmdArgs) < 2 {
		fmt.Fprintf(os.Stderr, "Error: %v\n", ErrMissingSetArguments)
		fmt.Fprintf(os.Stderr, "Usage: ... set <path> <value>\n")
		os.Exit(1)
	}

	configPath := cmdArgs[0]
	rawValue := cmdArgs[1]

	if err := setConfigValue(os.Stdout, secureCfg, scope, format, description, configPath, rawValue); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// setConfigValue contains the testable core logic for setting a configuration value.
// It accepts io.Writer for output, making it easy to test.
func setConfigValue(
	stdout io.Writer,
	secureCfg config.SecureStore,
	scope string,
	format string,
	description string,
	configPath string,
	rawValue string) error {

	if scope == "" {
		scope = config.ScopeApplication
	}

	decryptedData, fileFormat, err := secureCfg.Get(scope, 0) // generation 0 = latest
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve latest config for scope '%s': %w", ErrSecureStoreGet, scope, err)
	}

	tree, err := toml.LoadBytes(decryptedData)
	if err != nil {
		return fmt.Errorf("%w: failed to load config data for scope '%s': %w", ErrConfigUnmarshal, scope, err)
	}

	var valueToSet interface{}
	if strings.HasPrefix(rawValue, "@") {
		filePath := strings.TrimPrefix(rawValue, "@")
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("%w: failed to read from path '%s': %w", ErrReadFile, filePath, err)
		}
		valueToSet = string(fileContent)
	} else {
		tempTomlString := fmt.Sprintf("temp_key = %s", rawValue)
		tempTree, err := toml.Load(tempTomlString)
		if err != nil {
			tempTomlString = fmt.Sprintf("temp_key = %q", rawValue)
			tempTree, err = toml.Load(tempTomlString)
			if err != nil {
				return fmt.Errorf("%w: could not parse '%s': %w", ErrParseValue, rawValue, err)
			}
		}
		valueToSet = tempTree.Get("temp_key")
	}

	if !tree.Has(configPath) {
		return fmt.Errorf("%w: path '%s' not found in config for scope '%s'", ErrPathNotFound, configPath, scope)
	}

	tree.Set(configPath, valueToSet)

	updatedTomlBytes, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal updated config: %w", ErrConfigMarshal, err)
	}

	if description == "" {
		description = fmt.Sprintf("Updated field '%s'", configPath)
	}

	// Preserve the original format from the file unless overridden by the flag
	saveFormat := fileFormat
	if format != "" {
		saveFormat = format
	}

	err = secureCfg.Save(scope, updatedTomlBytes, saveFormat, description)
	if err != nil {
		return fmt.Errorf("%w: failed to save updated config for scope '%s': %w", ErrSecureStoreSave, scope, err)
	}

	if _, err := fmt.Fprintf(stdout, "Successfully set '%s' in scope '%s'\n", configPath, scope); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}
