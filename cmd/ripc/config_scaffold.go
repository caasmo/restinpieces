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
	ErrScaffoldParentMissing = errors.New("parent section not found in config; run 'config migrate' first")
)

const (
	ScaffoldTypeBackupLocal = "backuplocal"
	ScaffoldTypeOAuth2      = "oauth2"

	scaffoldKeyBackupLocal = "backup_local.files"
	scaffoldKeyOAuth2      = "oauth2_providers"

	scaffoldParentBackupLocal = "backup_local"
)

var knownScaffoldTypes = []string{ScaffoldTypeBackupLocal, ScaffoldTypeOAuth2}

func scaffoldDefaults(scaffoldType string) (tomlKey string, parentPath string, defaults interface{}, err error) {
	switch scaffoldType {
	case ScaffoldTypeBackupLocal:
		return scaffoldKeyBackupLocal, scaffoldParentBackupLocal, config.NewBackupLocalDbFileDefaults(), nil
	case ScaffoldTypeOAuth2:
		return scaffoldKeyOAuth2, scaffoldKeyOAuth2, config.NewOAuth2ProviderDefaults(), nil
	default:
		return "", "", nil, fmt.Errorf("%w: '%s'. Known types: %s",
			ErrScaffoldTypeUnknown, scaffoldType,
			strings.Join(knownScaffoldTypes, ", "))
	}
}

func printConfigScaffoldUsage(w io.Writer) {
	help := CommandHelp{
		Usage:       "ripc config scaffold [options] <type> <key>",
		Description: "Scaffolds a new configuration entry with sensible defaults under the given type and key. Requires the parent config section to exist — run 'config migrate' first if needed.",
		Subcommands: []SubcommandGroup{
			{
				Title: "Scaffold Types",
				Subcommands: []Subcommand{
					{"backuplocal", "Scaffold a backup_local.files entry"},
					{"oauth2", "Scaffold an oauth2_providers entry"},
				},
			},
		},
		Options: map[string]Option{
			"scope": commandConfig.Options["scope"],
			"desc":  commandConfig.Options["desc"],
		},
		Examples: []string{
			`ripc config scaffold backuplocal app_db`,
			`ripc config scaffold oauth2 my_google`,
			`ripc config scaffold --scope my-app backuplocal analytics_db`,
			`ripc config scaffold --scope my-app --desc "scaffold analytics db" backuplocal analytics_db`,
		},
	}
	help.Print(w, "ripc", "config", "scaffold")
}

func handleConfigScaffoldCommand(secureStore config.SecureStore, opts ConfigScaffoldOptions, ui UI) error {
	return scaffoldConfigValue(ui.Out, secureStore, opts.Scope, opts.Desc, opts.ScaffoldType, opts.Key)
}

func scaffoldConfigValue(
	stdout io.Writer,
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
		return fmt.Errorf("%w: '%s' not found in scope '%s'; run 'ripc config migrate' to initialize missing config sections",
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

	_, err = fmt.Fprintf(stdout, "Successfully scaffolded '%s' in scope '%s'\n", configPath, scope)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	return nil
}

// ConfigScaffoldOptions holds the parsed options for the 'config scaffold' subcommand.
type ConfigScaffoldOptions struct {
	Scope        string // --scope
	Desc         string // --desc
	ScaffoldType string // positional type argument
	Key          string // positional key argument
}

// parseConfigScaffoldArgs parses the arguments for the 'scaffold' subcommand.
func parseConfigScaffoldArgs(args []string) (ConfigScaffoldOptions, error) {
	scaffoldCmd := flag.NewFlagSet("scaffold", flag.ContinueOnError)
	scaffoldCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]
	descOpt := commandConfig.Options["desc"]

	var opts ConfigScaffoldOptions
	scaffoldCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	scaffoldCmd.StringVar(&opts.Desc, "desc", descOpt.DefaultValue, descOpt.Usage)

	err := scaffoldCmd.Parse(args)
	if err != nil {
		return ConfigScaffoldOptions{}, fmt.Errorf("parsing scaffold flags: %w", err)
	}
	if scaffoldCmd.NArg() < 2 {
		return ConfigScaffoldOptions{}, fmt.Errorf("'scaffold' requires <type> and <key> arguments: %w", ErrMissingArgument)
	}
	if scaffoldCmd.NArg() > 2 {
		return ConfigScaffoldOptions{}, fmt.Errorf("'scaffold' takes exactly two arguments: type and key: %w", ErrTooManyArguments)
	}
	opts.ScaffoldType = scaffoldCmd.Arg(0)
	opts.Key = scaffoldCmd.Arg(1)
	return opts, nil
}
