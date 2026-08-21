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
	ScaffoldTypeBackupOnlineAPI   = "backup-online-api"
	ScaffoldTypeBackupVacuum      = "backup-vacuum"
	ScaffoldTypeBackupSqliteRsync = "backup-sqlite-rsync"
	ScaffoldTypeOAuth2            = "oauth2"

	scaffoldKeyBackup  = "backup.files"
	scaffoldKeyOAuth2  = "oauth2_providers"
	scaffoldParentBackup = "backup"
)

var knownScaffoldTypes = []string{ScaffoldTypeBackupOnlineAPI, ScaffoldTypeBackupVacuum, ScaffoldTypeBackupSqliteRsync, ScaffoldTypeOAuth2}

func scaffoldDefaults(scaffoldType string) (tomlKey string, parentPath string, defaults interface{}, err error) {
	switch scaffoldType {
	case ScaffoldTypeBackupOnlineAPI:
		return scaffoldKeyBackup, scaffoldParentBackup, config.NewBackupOnlineDefaults(), nil
	case ScaffoldTypeBackupVacuum:
		return scaffoldKeyBackup, scaffoldParentBackup, config.NewBackupVacuumDefaults(), nil
	case ScaffoldTypeBackupSqliteRsync:
		return scaffoldKeyBackup, scaffoldParentBackup, config.NewBackupSqliteRsyncDefaults(), nil
	case ScaffoldTypeOAuth2:
		return scaffoldKeyOAuth2, scaffoldKeyOAuth2, config.NewOAuth2ProviderDefaults(), nil
	default:
		return "", "", nil, fmt.Errorf("%w: '%s'. Known types: %s",
			ErrScaffoldTypeUnknown, scaffoldType,
			strings.Join(knownScaffoldTypes, ", "))
	}
}

// defaultFieldsAndValues returns the indented TOML block for the given
// defaults struct (the file's fields and values as they will be stored).
//
// It marshals the defaults struct (e.g. NewBackupVacuumDefaults()) with
// pelletier/go-toml, trims surrounding whitespace, and prefixes each line
// with two spaces so the block aligns under the label header:
//
//   label:
//     source_path = ""
//     strategy = "vacuum"
//     frequency = "15m"
//
// This is the single source of truth for the values shown in
// scaffoldNextSteps — the literal stays, the values do not.
//
// Example:
//
//   NewBackupSqliteRsyncDefaults:
//     source_path = ""
//     strategy = "sqlite-rsync"
//     sync_timeout = "15m"
func defaultFieldsAndValues(defaults interface{}) string {
	b, _ := toml.Marshal(defaults)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}

func scaffoldNextSteps(scaffoldType, label string, defaults interface{}) string {
	block := defaultFieldsAndValues(defaults)
	switch scaffoldType {
	case ScaffoldTypeBackupSqliteRsync:
		return fmt.Sprintf(`
%s:
%s

Next steps:
1. Set the origin file to replicate (required):
	ripc set backup.files.%s.source_path /path/to/app.db
2. Reload the app:
	systemctl reload myapp
Deactivate: ripc set backup.files.%s.source_path ""`, label, block, label, label)
	case ScaffoldTypeBackupVacuum:
		return fmt.Sprintf(`
%s:
%s

Next steps:
1. Set the origin file to back up (required):
	ripc set backup.files.%s.source_path /path/to/app.db
2. Set the backup destination directory (required):
	ripc set backup.files.%s.dest_path /var/backups
3. Optionally modify above values (frequency, compression) as needed:
	ripc set backup.files.%s.frequency 24h
4. Reload the app:
	systemctl reload myapp
Deactivate: ripc set backup.files.%s.source_path ""`, label, block, label, label, label, label)
	case ScaffoldTypeBackupOnlineAPI:
		return fmt.Sprintf(`
%s:
%s

Next steps:
1. Set the origin file to back up (required):
	ripc set backup.files.%s.source_path /path/to/app.db
2. Set the backup destination directory (required):
	ripc set backup.files.%s.dest_path /var/backups
3. Optionally modify above values (frequency, compression, tuning) as needed:
	ripc set backup.files.%s.frequency 24h
4. Reload the app:
	systemctl reload myapp
Deactivate: ripc set backup.files.%s.source_path ""`, label, block, label, label, label, label)
	default:
		return ""
	}
}

func printScaffoldUsage(w io.Writer) {
	help := Spec{
		Usage:       "scaffold [options] <type> <key>",
		Description: "Scaffolds a new configuration entry with sensible defaults under the given type and key. Requires the parent config section to exist — run 'migrate' first if needed. The key is required and becomes backup.files.<key>; use a best-practice label <dbfile>-<strategy> (e.g. app-online, analytics-vacuum, app-rsync) so the map key reveals the database and the engine.",
		Args: []ArgSpec{
			{"type", "Scaffold type (backup-online-api, backup-vacuum, backup-sqlite-rsync or oauth2)"},
			{"key", "Key of the new entry — required backup label, e.g. app-online, app-vacuum, app-rsync (file + method)"},
		},
		Subcommands: []SubcommandGroup{
			{
				Title: "Scaffold Types",
				Subcommands: []Subcommand{
					{"backup-online-api", "Scaffold a backup.files entry for Online API (non-blocking)"},
					{"backup-vacuum", "Scaffold a backup.files entry for VACUUM INTO (blocking)"},
					{"backup-sqlite-rsync", "Scaffold a backup.files entry for sqlite-rsync (origin serve)"},
					{"oauth2", "Scaffold an oauth2_providers entry"},
				},
			},
		},
		Options: []OptSpec{
			commandOptions.Opt("desc"),
		},
		Examples: []string{
			"ripc scaffold backup-online-api app-online",
			"ripc scaffold backup-vacuum app-vacuum",
			"ripc scaffold backup-sqlite-rsync app-rsync",
			"ripc scaffold oauth2 my_google",
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
	return scaffoldConfigValue(ui, secureStore, opts.Desc, opts.ScaffoldType, opts.Key)
}

func scaffoldConfigValue(
	ui UI,
	secureCfg config.SecureStore,
	description string,
	scaffoldType string,
	key string,
) error {
	if strings.ContainsAny(key, " \t\r\n.") {
		return fmt.Errorf("invalid scaffold key %q: must not contain whitespace or '.': %w", key, ErrInvalidFlag)
	}
	scope := config.ScopeApplication
	tomlKey, parentPath, defaults, err := scaffoldDefaults(scaffoldType)
	if err != nil {
		return err
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
	_, err = fmt.Fprintf(ui.Err, "Successfully scaffolded backup '%s' in scope '%s'\n", key, scope)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	nextSteps := scaffoldNextSteps(scaffoldType, key, defaults)
	if nextSteps != "" {
		_, err = fmt.Fprintf(ui.Err, "%s\n", nextSteps)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrWriteOutput, err)
		}
	}
	return nil
}

// ScaffoldOptions holds the parsed options for the 'scaffold' command.
type ScaffoldOptions struct {
	Desc         string // --desc
	ScaffoldType string // positional type argument
	Key          string // positional key argument
}

// parseScaffoldArgs parses the arguments for the 'scaffold' command.
// It does not support --scope — scaffold always writes to ScopeApplication.
func parseScaffoldArgs(args []string) (ScaffoldOptions, error) {
	scaffoldCmd := flag.NewFlagSet("scaffold", flag.ContinueOnError)
	scaffoldCmd.SetOutput(io.Discard)
	descOpt := commandOptions.Opt("desc")
	var opts ScaffoldOptions
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
	if strings.ContainsAny(opts.Key, " \t\r\n.") {
		return ScaffoldOptions{}, fmt.Errorf("invalid scaffold key %q: must not contain whitespace or '.': %w", opts.Key, ErrInvalidFlag)
	}
	return opts, nil
}
