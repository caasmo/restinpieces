package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml"
)

// Error definitions for set command
var (
	ErrPathNotFound      = errors.New("configuration path does not exist")
	ErrReadFile          = errors.New("failed to read value from file")
	ErrParseValue        = errors.New("failed to parse value")
	ErrUnsupportedFormat = errors.New("unsupported format")
)

func printSetUsage(w io.Writer) {
	help := Spec{
		Usage:       "set [options] <path> <value>",
		Description: "Sets a configuration value at a specified path.",
		Args: []ArgSpec{
			{"path", "Configuration path to set"},
			{"value", "Value to set"},
		},
		Options: []OptSpec{
			commandOptions.Opt("scope"),
			commandOptions.Opt("format"),
			commandOptions.Opt("desc"),
		},
		Examples: []string{
			"ripc set server.host localhost",
			`ripc set --scope webapp features.beta true --desc "Enable beta feature"`,
		},
	}
	help.Print(w, prog)
}

// handleSetCommand parses the arguments for the 'set' command and executes
// the core logic, returning any error to the caller.
func handleSetCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parseSetArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printSetUsage(ui.Out)
			return nil
		}
		printSetUsage(ui.Err)
		return err
	}
	return setConfigValue(ui, secureStore, opts.Scope, opts.Format, opts.Desc, opts.Path, opts.Value)
}

// setConfigValue contains the testable core logic for setting a configuration value.
// It accepts UI for output, making it easy to test.
func setConfigValue(
	ui UI,
	secureCfg config.SecureStore,
	scope string,
	format string,
	description string,
	configPath string,
	rawValue string) error {

	supportedFormats := []string{"toml"}
	isSupported := false
	for _, supported := range supportedFormats {
		if format == supported {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return fmt.Errorf("%w: '%s'. Supported formats are: %s", ErrUnsupportedFormat, format, strings.Join(supportedFormats, ", "))
	}

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

	if _, err := fmt.Fprintf(ui.Err, "Successfully set '%s' in scope '%s'\n", configPath, scope); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}

// SetOptions holds the parsed options for the 'set' command.
type SetOptions struct {
	Scope  string // --scope
	Format string // --format
	Desc   string // --desc
	Path   string // positional path argument
	Value  string // positional value argument
}

// parseSetArgs parses the arguments for the 'set' command.
func parseSetArgs(args []string) (SetOptions, error) {
	setCmd := flag.NewFlagSet("set", flag.ContinueOnError)
	setCmd.SetOutput(io.Discard) // Output not needed for parsing
	scopeOpt := commandOptions.Opt("scope")
	formatOpt := commandOptions.Opt("format")
	descOpt := commandOptions.Opt("desc")

	var opts SetOptions
	setCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	setCmd.StringVar(&opts.Format, "format", formatOpt.DefaultValue, formatOpt.Usage)
	setCmd.StringVar(&opts.Desc, "desc", descOpt.DefaultValue, descOpt.Usage)

	err := setCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return SetOptions{}, flag.ErrHelp
		}
		return SetOptions{}, fmt.Errorf("parsing set flags: %w: %v", ErrInvalidFlag, err)
	}
	if setCmd.NArg() < 2 {
		return SetOptions{}, fmt.Errorf("'set' requires path and value arguments: %w", ErrMissingArgument)
	}
	if setCmd.NArg() > 2 {
		return SetOptions{}, fmt.Errorf("'set' command takes at most two arguments (path and value): %w", ErrTooManyArguments)
	}
	opts.Path = setCmd.Arg(0)
	opts.Value = setCmd.Arg(1)
	return opts, nil
}
