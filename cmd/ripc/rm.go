package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml"
)

// ErrValueNotFound reports an array value that remove did not find.
var ErrValueNotFound = errors.New("value not found in array")

func printRmUsage(w io.Writer) {
	help := Spec{
		Usage:       "rm [options] <path> [value]",
		Description: "Removes one item from a configuration collection. A table is named by its path, for example 'backup.vacuum.logs-vacuum'. An array item is named by value, for example 'block_user_agent.agents SemrushBot'. Scalar keys like 'server.addr' are refused.",
		Args: []ArgSpec{
			{"path", "Configuration path of the table or array"},
			{"value", "Value to remove from an array"},
		},
		Options: []OptSpec{
			commandOptions.Opt("scope"),
			commandOptions.Opt("desc"),
		},
		Examples: []string{
			"ripc rm backup.vacuum.logs-vacuum",
			"ripc rm block_user_agent.agents SemrushBot",
		},
	}
	help.Print(w, prog)
}

// RmOptions holds the parsed options for the 'rm' command.
type RmOptions struct {
	Scope string // --scope
	Desc  string // --desc
	Path  string // positional path argument
	Value string // optional positional value argument
}

// handleRmCommand parses the arguments for the 'rm' command and calls
// removeValue, returning any error to the caller.
func handleRmCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parseRmArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printRmUsage(ui.Out)
			return nil
		}
		printRmUsage(ui.Err)
		return err
	}
	return removeValue(ui, secureStore, opts.Scope, opts.Desc, opts.Path, opts.Value)
}

// parseRmArgs parses the arguments for the 'rm' command.
func parseRmArgs(args []string) (RmOptions, error) {
	rmCmd := flag.NewFlagSet("rm", flag.ContinueOnError)
	rmCmd.SetOutput(io.Discard)
	scopeOpt := commandOptions.Opt("scope")
	descOpt := commandOptions.Opt("desc")

	var opts RmOptions
	rmCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	rmCmd.StringVar(&opts.Desc, "desc", descOpt.DefaultValue, descOpt.Usage)

	err := rmCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return RmOptions{}, flag.ErrHelp
		}
		return RmOptions{}, fmt.Errorf("parsing rm flags: %w: %v", ErrInvalidFlag, err)
	}
	if rmCmd.NArg() < 1 {
		return RmOptions{}, fmt.Errorf("'rm' requires path argument: %w", ErrMissingArgument)
	}
	if rmCmd.NArg() > 2 {
		return RmOptions{}, fmt.Errorf("'rm' command takes at most two arguments (path and value): %w", ErrTooManyArguments)
	}
	opts.Path = rmCmd.Arg(0)
	if rmCmd.NArg() == 2 {
		opts.Value = rmCmd.Arg(1)
	}
	return opts, nil
}

// removeTableParents lists the parent tables whose entries can be removed. An
// entry is named by the parent path plus the entry label, for example
// backup.vacuum.logs-vacuum.
var removeTableParents = map[string]struct{}{
	"backup.online":               {},
	"backup.vacuum":               {},
	"backup.sqlite-rsync.entries": {},
	"backup.s3-upload":            {},
	"backup.s3-download":          {},
	"scheduler.jobs":              {},
	"oauth2":                      {},
	"acme.dns-01":                 {},
}

// removeArrayPaths lists the keys that hold removable array items. An item is
// named by the key plus its value, for example block_user_agent.agents
// SemrushBot.
var removeArrayPaths = map[string]struct{}{
	"block_user_agent.agents":                {},
	"block_host.allowed_hosts":               {},
	"block_oversized_request.excluded_paths": {},
	"acme.domains":                           {},
}

// removeValue removes one item from a configuration collection; it is
// separate from the command so it can be tested directly.
func removeValue(ui UI, secureCfg config.SecureStore, scope string, description string, tomlPath string, value string) error {
	if scope == "" {
		scope = config.ScopeApplication
	}

	decryptedData, fileFormat, err := secureCfg.Get(scope, 0)
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve latest config for scope '%s': %w", ErrSecureStoreGet, scope, err)
	}

	parser, err := NewTomlParser(decryptedData, "")
	if err != nil {
		return fmt.Errorf("%w: failed to load config data for scope '%s': %w", ErrConfigUnmarshal, scope, err)
	}

	tree, _, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("%w: failed to load config data for scope '%s': %w", ErrConfigUnmarshal, scope, err)
	}

	if !tree.Has(tomlPath) {
		return fmt.Errorf("%w: path '%s' not found in config for scope '%s'", ErrPathNotFound, tomlPath, scope)
	}

	removeErr := removeItem(tree, tomlPath, value)
	if removeErr != nil {
		return removeErr
	}

	updatedTomlBytes, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal updated config: %w", ErrConfigMarshal, err)
	}

	validationErr := validateTomlAsConfig(updatedTomlBytes, tomlPath)
	if validationErr != nil {
		return validationErr
	}

	removedItem := fmt.Sprintf("'%s'", tomlPath)
	if value != "" {
		removedItem = fmt.Sprintf("'%s' from '%s'", value, tomlPath)
	}

	if description == "" {
		description = "Removed " + removedItem
	}

	err = secureCfg.Save(scope, updatedTomlBytes, fileFormat, description)
	if err != nil {
		return fmt.Errorf("%w: failed to save updated config for scope '%s': %w", ErrSecureStoreSave, scope, err)
	}

	_, err = fmt.Fprintf(ui.Err, "Successfully removed %s in scope '%s'\n", removedItem, scope)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}

// removeItem drops one item from the collection at tomlPath. An array path
// names the item by value; a parent table names it by the last path segment.
func removeItem(tree *toml.Tree, tomlPath, value string) error {
	if _, ok := removeArrayPaths[tomlPath]; ok {
		if value == "" {
			return fmt.Errorf("%w: '%s' needs a value to remove", ErrMissingArgument, tomlPath)
		}
		return removeArrayItem(tree, tomlPath, value)
	}

	parentPath := parentTomlKeyOf(tomlPath)
	if _, ok := removeTableParents[parentPath]; !ok {
		return fmt.Errorf("%w: path '%s' is not removable", ErrNotCollection, tomlPath)
	}
	if value != "" {
		return fmt.Errorf("%w: '%s' takes no value", ErrTooManyArguments, tomlPath)
	}
	return tree.Delete(tomlPath)
}

// removeArrayItem removes the first item equal to value from the TOML array
// at tomlPath.
func removeArrayItem(tree *toml.Tree, tomlPath, value string) error {
	raw, ok := tomlArrayOf(tree.Get(tomlPath))
	if !ok {
		return fmt.Errorf("%w: %s is not an array", ErrNotCollection, tomlPath)
	}

	for i, item := range raw {
		str, ok := item.(string)
		if !ok || str != value {
			continue
		}
		SetWithComment(tree, tomlPath, append(raw[:i], raw[i+1:]...))
		return nil
	}
	return fmt.Errorf("%w: '%s' in '%s'", ErrValueNotFound, value, tomlPath)
}
