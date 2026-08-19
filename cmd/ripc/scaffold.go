package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml"
)

var (
	ErrScaffoldTypeUnknown   = errors.New("unknown scaffold type")
	ErrScaffoldKeyExists     = errors.New("key already exists for scaffold type")
	ErrScaffoldParentMissing = errors.New("parent section not found in config")
)

const (
	ScaffoldTypeBackup = "backup"
	ScaffoldTypeOAuth2 = "oauth2"

	scaffoldKeyBackup = "backup.files"
	scaffoldKeyOAuth2 = "oauth2_providers"

	scaffoldParentBackup = "backup"
)

var knownScaffoldTypes = []string{ScaffoldTypeBackup, ScaffoldTypeOAuth2}

func scaffoldDefaults(scaffoldType string) (tomlKey string, parentPath string, defaults interface{}, err error) {
	switch scaffoldType {
	case ScaffoldTypeBackup:
		return scaffoldKeyBackup, scaffoldParentBackup, config.NewBackupFileDefaults(), nil
	case ScaffoldTypeOAuth2:
		return scaffoldKeyOAuth2, scaffoldKeyOAuth2, config.NewOAuth2ProviderDefaults(), nil
	default:
		return "", "", nil, fmt.Errorf("%w: '%s'. Known types: %s",
			ErrScaffoldTypeUnknown, scaffoldType,
			strings.Join(knownScaffoldTypes, ", "))
	}
}

func printScaffoldUsage(w io.Writer) {
	help := Spec{
		Usage:       "scaffold [options] <type> <key>",
		Description: "Scaffolds a new configuration entry with sensible defaults under the given type and key. Requires the parent config section to exist — run 'migrate' first if needed.",
		Args: []ArgSpec{
			{"type", "Scaffold type (backup or oauth2)"},
			{"key", "Key of the new entry"},
		},
		Subcommands: []SubcommandGroup{
			{
				Title: "Scaffold Types",
				Subcommands: []Subcommand{
					{"backup", "Scaffold a backup.files entry"},
					{"oauth2", "Scaffold an oauth2_providers entry"},
				},
			},
		},
		Options: []OptSpec{
			commandOptions.Opt("scope"),
			commandOptions.Opt("desc"),
		},
		Examples: []string{
			"ripc scaffold backup app_db",
			"ripc scaffold oauth2 my_google",
			"ripc scaffold --scope my-app backup analytics_db",
			"ripc scaffold --scope my-app --desc \"scaffold analytics db\" backup analytics_db",
		},
	}
	help.Print(w, prog)
}

// handleScaffoldCommand parses the arguments for the 'scaffold' command and
// executes the core logic, returning any error to the caller.
func handleScaffoldCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parseScaffoldArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printScaffoldUsage(ui.Out)
			return nil
		}
		printScaffoldUsage(ui.Err)
		return err
	}
	return scaffoldConfigValue(ui, secureStore, opts.Scope, opts.Desc, opts.ScaffoldType, opts.Key)
}

func scaffoldConfigValue(
	ui UI,
	secureCfg config.SecureStore,
	scope string,
	description string,
	scaffoldType string,
	key string,
) error {
	tomlKey, parentPath, defaults, err := scaffoldDefaults(scaffoldType)
	if err != nil {
		return err
	}

	if scope == "" {
		scope = config.ScopeApplication
	}

	decryptedData, fileFormat, err := secureCfg.Get(scope, 0)
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve latest config for scope '%s': %w",
			ErrSecureStoreGet, scope, err)
	}

	tree, err := toml.LoadBytes(decryptedData)
	if err != nil {
		return fmt.Errorf("%w: failed to load config data for scope '%s': %w",
			ErrConfigUnmarshal, scope, err)
	}

	if !tree.Has(parentPath) {
		return fmt.Errorf("%w: '%s' not found in scope '%s'; run 'ripc migrate' to initialize missing config sections",
			ErrScaffoldParentMissing, parentPath, scope)
	}

	configPath := tomlKey + "." + key
	if tree.Has(configPath) {
		return fmt.Errorf("%w: '%s' already exists in scope '%s'",
			ErrScaffoldKeyExists, configPath, scope)
	}

	subtreeBytes, err := toml.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal scaffold defaults: %w",
			ErrConfigMarshal, err)
	}

	subtree, err := toml.LoadBytes(subtreeBytes)
	if err != nil {
		return fmt.Errorf("%w: failed to load scaffold subtree: %w",
			ErrConfigUnmarshal, err)
	}

	tree.Set(configPath, subtree)

	updatedTomlBytes, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal updated config: %w",
			ErrConfigMarshal, err)
	}

	if description == "" {
		description = fmt.Sprintf("Scaffolded '%s'", configPath)
	}

	err = secureCfg.Save(scope, updatedTomlBytes, fileFormat, description)
	if err != nil {
		return fmt.Errorf("%w: failed to save updated config for scope '%s': %w",
			ErrSecureStoreSave, scope, err)
	}

	_, err = fmt.Fprintf(ui.Err, "Successfully scaffolded '%s' in scope '%s'\n", configPath, scope)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	return nil
}

// ScaffoldOptions holds the parsed options for the 'scaffold' command.
type ScaffoldOptions struct {
	Scope        string // --scope
	Desc         string // --desc
	ScaffoldType string // positional type argument
	Key          string // positional key argument
}

// parseScaffoldArgs parses the arguments for the 'scaffold' command.
func parseScaffoldArgs(args []string) (ScaffoldOptions, error) {
	scaffoldCmd := flag.NewFlagSet("scaffold", flag.ContinueOnError)
	scaffoldCmd.SetOutput(io.Discard)
	scopeOpt := commandOptions.Opt("scope")
	descOpt := commandOptions.Opt("desc")

	var opts ScaffoldOptions
	scaffoldCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	scaffoldCmd.StringVar(&opts.Desc, "desc", descOpt.DefaultValue, descOpt.Usage)

	err := scaffoldCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ScaffoldOptions{}, flag.ErrHelp
		}
		return ScaffoldOptions{}, fmt.Errorf("parsing scaffold flags: %w: %v", ErrInvalidFlag, err)
	}
	if scaffoldCmd.NArg() < 2 {
		return ScaffoldOptions{}, fmt.Errorf("'scaffold' requires <type> and <key> arguments: %w", ErrMissingArgument)
	}
	if scaffoldCmd.NArg() > 2 {
		return ScaffoldOptions{}, fmt.Errorf("'scaffold' takes exactly two arguments: type and key: %w", ErrTooManyArguments)
	}
	opts.ScaffoldType = scaffoldCmd.Arg(0)
	opts.Key = scaffoldCmd.Arg(1)
	return opts, nil
}
