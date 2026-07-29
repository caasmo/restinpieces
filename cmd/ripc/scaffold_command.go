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

var (
	ErrScaffoldTypeUnknown = errors.New("unknown scaffold type")
	ErrScaffoldKeyExists   = errors.New("key already exists for scaffold type")
)

const (
	ScaffoldTypeBackupLocal = "backuplocal"
	ScaffoldTypeOAuth2      = "oauth2"

	scaffoldKeyBackupLocal = "backup_local.files"
	scaffoldKeyOAuth2      = "oauth2_providers"
)

var knownScaffoldTypes = []string{ScaffoldTypeBackupLocal, ScaffoldTypeOAuth2}

func scaffoldDefaults(scaffoldType string) (tomlKey string, defaults interface{}, err error) {
	switch scaffoldType {
	case ScaffoldTypeBackupLocal:
		return scaffoldKeyBackupLocal, config.NewBackupLocalDbFileDefaults(), nil
	case ScaffoldTypeOAuth2:
		return scaffoldKeyOAuth2, config.NewOAuth2ProviderDefaults(), nil
	default:
		return "", nil, fmt.Errorf("%w: '%s'. Known types: %s",
			ErrScaffoldTypeUnknown, scaffoldType,
			strings.Join(knownScaffoldTypes, ", "))
	}
}

func printScaffoldUsage() {
	help := CommandHelp{
		Usage:       "ripc config scaffold [options] <type> <key>",
		Description: "Scaffolds a new configuration entry with sensible defaults under the given type and key.",
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
	help.Print(os.Stderr, "ripc", "config", "scaffold")
}

func handleScaffoldCommand(secureStore config.SecureStore, scope, desc, scaffoldType, key string) {
	err := scaffoldConfigValue(os.Stdout, secureStore, scope, desc, scaffoldType, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func scaffoldConfigValue(
	stdout io.Writer,
	secureCfg config.SecureStore,
	scope string,
	description string,
	scaffoldType string,
	key string,
) error {
	tomlKey, defaults, err := scaffoldDefaults(scaffoldType)
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
