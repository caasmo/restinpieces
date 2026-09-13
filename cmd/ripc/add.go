package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml"
)

// ErrNotCollection reports a path that has no adder.
var ErrNotCollection = errors.New("unsupported path")

func printAddUsage(w io.Writer) {
	help := Spec{
		Usage:       "add [options] <path> <value>",
		Description: "Appends a value to a configuration key that holds several values.",
		Args: []ArgSpec{
			{"path", "Configuration path of the collection"},
			{"value", "Value to append"},
		},
		Options: []OptSpec{
			commandOptions.Opt("scope"),
			commandOptions.Opt("desc"),
		},
		Examples: []string{
			"ripc add block_ua_list.list SemrushBot",
		},
	}
	help.Print(w, prog)
}

// AddOptions holds the parsed options for the 'add' command.
type AddOptions struct {
	Scope string // --scope
	Desc  string // --desc
	Path  string // positional path argument
	Value string // positional value argument
}

// handleAddCommand parses the arguments for the 'add' command and calls
// addValue, returning any error to the caller.
func handleAddCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parseAddArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printAddUsage(ui.Out)
			return nil
		}
		printAddUsage(ui.Err)
		return err
	}
	return addValue(ui, secureStore, opts.Scope, opts.Desc, opts.Path, opts.Value)
}

// parseAddArgs parses the arguments for the 'add' command.
func parseAddArgs(args []string) (AddOptions, error) {
	addCmd := flag.NewFlagSet("add", flag.ContinueOnError)
	addCmd.SetOutput(io.Discard)
	scopeOpt := commandOptions.Opt("scope")
	descOpt := commandOptions.Opt("desc")

	var opts AddOptions
	addCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	addCmd.StringVar(&opts.Desc, "desc", descOpt.DefaultValue, descOpt.Usage)

	err := addCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return AddOptions{}, flag.ErrHelp
		}
		return AddOptions{}, fmt.Errorf("parsing add flags: %w: %v", ErrInvalidFlag, err)
	}
	if addCmd.NArg() < 2 {
		return AddOptions{}, fmt.Errorf("'add' requires path and value arguments: %w", ErrMissingArgument)
	}
	if addCmd.NArg() > 2 {
		return AddOptions{}, fmt.Errorf("'add' command takes at most two arguments (path and value): %w", ErrTooManyArguments)
	}
	opts.Path = addCmd.Arg(0)
	opts.Value = addCmd.Arg(1)
	return opts, nil
}

// adder adds one value to a configuration value.
type adder interface {
	Add(existing interface{}, value string) (interface{}, error)
}

// addFuncs maps addable configuration paths to their adder.
var addFuncs = map[string]adder{
	"block_ua_list.list": userAgentAdder{},
}

// addValue adds one value to a configuration key; it is separate from
// the command so it can be tested directly.
func addValue(ui UI, secureCfg config.SecureStore, scope string, description string, tomlPath string, value string) error {
	if scope == "" {
		scope = config.ScopeApplication
	}

	decryptedData, fileFormat, err := secureCfg.Get(scope, 0)
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve latest config for scope '%s': %w", ErrSecureStoreGet, scope, err)
	}

	tree, err := toml.LoadBytes(decryptedData)
	if err != nil {
		return fmt.Errorf("%w: failed to load config data for scope '%s': %w", ErrConfigUnmarshal, scope, err)
	}

	if !tree.Has(tomlPath) {
		return fmt.Errorf("%w: path '%s' not found in config for scope '%s'", ErrPathNotFound, tomlPath, scope)
	}

	adder, ok := addFuncs[tomlPath]
	if !ok {
		return fmt.Errorf("%w: path '%s' is not addable", ErrNotCollection, tomlPath)
	}

	existing := tree.Get(tomlPath)
	updated, err := adder.Add(existing, value)
	if err != nil {
		return err
	}

	tree.Set(tomlPath, updated)

	updatedTomlBytes, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal updated config: %w", ErrConfigMarshal, err)
	}

	if description == "" {
		description = fmt.Sprintf("Added to '%s'", tomlPath)
	}

	err = secureCfg.Save(scope, updatedTomlBytes, fileFormat, description)
	if err != nil {
		return fmt.Errorf("%w: failed to save updated config for scope '%s': %w", ErrSecureStoreSave, scope, err)
	}

	_, err = fmt.Fprintf(ui.Err, "Successfully added to '%s' in scope '%s'\n", tomlPath, scope)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}
