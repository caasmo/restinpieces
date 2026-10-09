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
	ScaffoldTypeBackupOnline      = "backup-online"
	ScaffoldTypeBackupVacuum      = "backup-vacuum"
	ScaffoldTypeBackupSqliteRsync = "backup-sqlite-rsync"
	ScaffoldTypeBackupS3Upload    = "backup-s3-upload"
	ScaffoldTypeBackupS3Download  = "backup-s3-download"
	ScaffoldTypeOAuth2            = "oauth2"
	ScaffoldTypeAcmeDNS01         = "acme-dns-01"
	ScaffoldTypeJob               = "job"
)

var knownScaffoldTypes = []string{ScaffoldTypeBackupOnline, ScaffoldTypeBackupVacuum, ScaffoldTypeBackupSqliteRsync, ScaffoldTypeBackupS3Upload, ScaffoldTypeBackupS3Download, ScaffoldTypeOAuth2, ScaffoldTypeAcmeDNS01, ScaffoldTypeJob}

func scaffoldDefaults(scaffoldType string) (tomlKey string, defaults interface{}, sectionDefaults interface{}, err error) {
	switch scaffoldType {
	case ScaffoldTypeBackupOnline:
		return "backup.online", config.NewBackupOnlineAPIEntryDefaults(), nil, nil
	case ScaffoldTypeBackupVacuum:
		return "backup.vacuum", config.NewBackupVacuumEntryDefaults(), nil, nil
	case ScaffoldTypeBackupSqliteRsync:
		return "backup.sqlite-rsync.entries", config.NewBackupSqliteRsyncEntryDefaults(), config.NewBackupSqliteRsyncDefaults(), nil
	case ScaffoldTypeBackupS3Upload:
		return "backup.s3-upload", config.NewBackupS3UploadEntryDefaults(), nil, nil
	case ScaffoldTypeBackupS3Download:
		return "backup.s3-download", config.NewBackupS3DownloadEntryDefaults(), nil, nil
	case ScaffoldTypeOAuth2:
		return "oauth2", config.NewOAuth2EntryDefaults(), nil, nil
	case ScaffoldTypeAcmeDNS01:
		return "acme.dns-01", config.NewAcmeDNS01EntryDefaults(), config.NewAcmeDefaults(), nil
	case ScaffoldTypeJob:
		return "scheduler.jobs", config.NewJobEntryDefaults(), nil, nil
	default:
		return "", nil, nil, fmt.Errorf("%w: '%s'. Known types: %s", ErrScaffoldTypeUnknown, scaffoldType, strings.Join(knownScaffoldTypes, ", "))
	}
}

// parentTomlKeyOf returns the parent path of a dot-separated TOML key, i.e.
// the prefix before the last dot. It is the single place that derives
// the parent key from a path.
//
// Examples:
//
//	parentTomlKeyOf("backup.online") == "backup"
//	parentTomlKeyOf("backup.sqlite-rsync.entries") == "backup.sqlite-rsync"
//	parentTomlKeyOf("oauth2") == ""
//	parentTomlKeyOf("backup") == ""
func parentTomlKeyOf(tomlKey string) string {
	idx := strings.LastIndex(tomlKey, ".")
	if idx == -1 {
		return ""
	}
	return tomlKey[:idx]
}

// defaultFieldsAndValues returns the indented TOML block for the given
// defaults struct (the file's fields and values as they will be stored).
//
// It marshals the defaults struct (e.g. NewBackupVacuumDefaults()) with
// pelletier/go-toml, trims surrounding whitespace, and prefixes each line
// with two spaces so the block aligns under the label header:
//
//	label:
//	  source_path = ""
//	  strategy = "vacuum"
//	  frequency = "15m"
//
// This is the single source of truth for the values shown in
// scaffoldNextSteps — the literal stays, the values do not.
//
// Example:
//
//	NewBackupSqliteRsyncDefaults:
//	  source_path = ""
//	  strategy = "sqlite-rsync"
//	  sync_timeout = "15m"
func defaultFieldsAndValues(defaults interface{}) string {
	b, _ := toml.Marshal(defaults)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}

// scaffoldNextSteps returns the closing instructions printed after an entry is
// scaffolded: the fields the operator now owns, the walk that fills them one at
// a time, the reload that applies them, and the command that turns the entry off.
//
// The walk filter is the entry path, so every key under the new entry is shown
// in turn with the comment written above it and its current value.
func scaffoldNextSteps(tomlKey string, label string, defaults interface{}) string {
	steps := fmt.Sprintf(`
%s:
%s

Set each value in turn:
	ripc walk %s.%s

Reload the app:
	systemctl reload myapp`, label, defaultFieldsAndValues(defaults), tomlKey, label)

	if deactivate := scaffoldDeactivate(tomlKey, label); deactivate != "" {
		steps += "\n" + deactivate
	}
	return steps
}

// scaffoldDeactivate returns the command that turns a scaffolded entry off. An
// empty path or prefix deactivates a backup entry, an empty provider an acme
// entry and false a job. It is empty for the types with nothing to turn off,
// such as oauth2.
func scaffoldDeactivate(tomlKey string, label string) string {
	switch tomlKey {
	case "backup.online", "backup.vacuum", "backup.sqlite-rsync.entries":
		return fmt.Sprintf("Deactivate: ripc set %s.%s.source_path \"\"", tomlKey, label)
	case "backup.s3-upload":
		return fmt.Sprintf("Deactivate: ripc set %s.%s.path \"\"\n	ripc set %s.%s.path_prefix \"\"", tomlKey, label, tomlKey, label)
	case "backup.s3-download":
		return fmt.Sprintf("Deactivate: ripc set %s.%s.object_key_prefix \"\"", tomlKey, label)
	case "acme.dns-01":
		return fmt.Sprintf("Deactivate: ripc set %s.%s.provider \"\"", tomlKey, label)
	case "scheduler.jobs":
		return fmt.Sprintf("Deactivate: ripc set %s.%s.activated false", tomlKey, label)
	}
	return ""
}

func printScaffoldUsage(w io.Writer) {
	help := Spec{
		Usage:       "scaffold [options] <type> <key>",
		Description: "Scaffolds a new configuration entry with sensible defaults under the given type and key. Requires the parent config section to exist — run 'migrate' first if needed. The key is required and becomes backup.online.<key>, backup.vacuum.<key>, backup.sqlite-rsync.entries.<key>, backup.s3-upload.<key>, backup.s3-download.<key>, acme.dns-01.<key> or scheduler.jobs.<key>; use a best-practice label that reveals what the entry is for (e.g. app-online, analytics-vacuum, app-rsync, my_cf).",
		Args: []ArgSpec{
			{"type", "Scaffold type (backup-online, backup-vacuum, backup-sqlite-rsync, backup-s3-upload, backup-s3-download, oauth2, acme-dns-01 or job)"},
			{"key", "Key of the new entry — required backup label, acme dns-01 label or job label, e.g. app-online, app-rsync, my_cf, acme_cert"},
		},
		Subcommands: []SubcommandGroup{
			{
				Title: "Scaffold Types",
				Subcommands: []Subcommand{
					{"backup-online", "Scaffold a backup.online entry for Online API (non-blocking)"},
					{"backup-vacuum", "Scaffold a backup.vacuum entry for VACUUM INTO (blocking)"},
					{"backup-sqlite-rsync", "Scaffold a backup.sqlite-rsync.entries entry for sqlite-rsync (origin serve)"},
					{"backup-s3-upload", "Scaffold a backup.s3-upload entry that uploads one file to S3"},
					{"backup-s3-download", "Scaffold a backup.s3-download entry that pulls the newest backup for a label from S3"},
					{"oauth2", "Scaffold an oauth2 entry"},
					{"acme-dns-01", "Scaffold an acme.dns-01 entry for the DNS-01 challenge"},
					{"job", "Scaffold a scheduler.jobs entry for a job that runs on a schedule"},
				},
			},
		},
		Options: []OptSpec{
			commandOptions.Opt("desc"),
		},
		Examples: []string{
			"ripc scaffold backup-online app-online",
			"ripc scaffold backup-vacuum app-vacuum",
			"ripc scaffold backup-sqlite-rsync app-rsync",
			"ripc scaffold backup-s3-upload app-s3",
			"ripc scaffold backup-s3-download app-dl",
			"ripc scaffold oauth2 my_google",
			"ripc scaffold acme-dns-01 my_cf",
			"ripc scaffold job acme_cert",
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
	tomlKey, defaults, sectionDefaults, err := scaffoldDefaults(scaffoldType)
	if err != nil {
		return err
	}
	parentPath := parentTomlKeyOf(tomlKey)
	if parentPath == "" {
		parentPath = tomlKey
	}
	decryptedData, fileFormat, err := secureCfg.Get(scope, 0)
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve latest config for scope '%s': %w",
			ErrSecureStoreGet, scope, err)
	}
	parser, err := NewTomlParser(decryptedData, "")
	if err != nil {
		return fmt.Errorf("%w: failed to load config data for scope '%s': %w",
			ErrConfigUnmarshal, scope, err)
	}

	tree, _, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("%w: failed to load config data for scope '%s': %w",
			ErrConfigUnmarshal, scope, err)
	}
	if !tree.Has(parentPath) {
		if sectionDefaults == nil {
			return fmt.Errorf("%w: '%s' not found in scope '%s'\nrun 'ripc migrate' to initialize missing config sections",
				ErrScaffoldParentMissing, parentPath, scope)
		}
		sectionBytes, err := toml.Marshal(sectionDefaults)
		if err != nil {
			return fmt.Errorf("%w: failed to marshal scaffold section defaults: %w", ErrConfigMarshal, err)
		}
		sectionParser, err := NewTomlParser(sectionBytes, "")
		if err != nil {
			return fmt.Errorf("%w: failed to load scaffold section defaults: %w", ErrConfigUnmarshal, err)
		}

		sectionTree, _, err := sectionParser.Parse()
		if err != nil {
			return fmt.Errorf("%w: failed to load scaffold section defaults: %w", ErrConfigUnmarshal, err)
		}
		tree.Set(parentPath, sectionTree)
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
	subtreeParser, err := NewTomlParser(subtreeBytes, "")
	if err != nil {
		return fmt.Errorf("%w: failed to load scaffold subtree: %w",
			ErrConfigUnmarshal, err)
	}

	subtree, _, err := subtreeParser.Parse()
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
	validationErr := validateTomlAsConfig(updatedTomlBytes, configPath)
	if validationErr != nil {
		return validationErr
	}
	if description == "" {
		description = fmt.Sprintf("Scaffolded '%s'", configPath)
	}
	err = secureCfg.Save(scope, updatedTomlBytes, fileFormat, description)
	if err != nil {
		return fmt.Errorf("%w: failed to save updated config for scope '%s': %w",
			ErrSecureStoreSave, scope, err)
	}
	_, err = fmt.Fprintf(ui.Err, "Successfully scaffolded '%s' in scope '%s'\n", key, scope)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	nextSteps := scaffoldNextSteps(tomlKey, key, defaults)
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
