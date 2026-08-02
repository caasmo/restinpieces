package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/caasmo/restinpieces/config"
	"zombiezen.com/go/sqlite/sqlitex"
)

var (
	ErrUnknownSubcommand = errors.New("unknown config subcommand")

	// commandConfig is the single source of truth for the 'ripc config' command's definition and help text.
	commandConfig = CommandHelp{
		Usage:       fmt.Sprintf("%s config <subcommand> [options]", prog),
		Description: "Manages application configuration with versioning and scope support.",
		Subcommands: []SubcommandGroup{
			{
				Title: "Reading Configuration",
				Subcommands: []Subcommand{
					{"get [filter]", "Get configuration values by path"},
					{"paths [filter]", "List all keys in the configuration"},
					{"dump", "Dump the configuration"},
					{"scopes", "List all configuration scopes"},
				},
			},
			{
				Title: "Modifying Configuration",
				Subcommands: []Subcommand{
					{"set <path> <value>", "Set a configuration value"},
					{"save <file>", "Save file contents to the configuration"},
					{"scaffold <type> <key>", "Scaffold a configuration entry with defaults"},
					{"migrate", "Migrate configuration to current framework version"},
				},
			},
			{
				Title: "Version Control",
				Subcommands: []Subcommand{
					{"list [scope]", "List configuration versions"},
					{"diff <generation>", "Compare configuration versions"},
					{"rollback <generation>", "Restore a previous configuration version"},
				},
			},
		},
		Options: map[string]Option{
			"scope":   {DefaultValue: config.ScopeApplication, Usage: "Scope for the configuration (affects: set, get, paths, dump, diff, rollback, save)"},
			"format":  {DefaultValue: "toml", Usage: "Format of the configuration file (affects: set, save)"},
			"desc":    {Usage: "Optional description for this configuration version (affects: set, save)"},
			"zero":    {Usage: "Output stored overrides on top of zero values (affects: dump)"},
			"runtime": {Usage: "Output defaults merged with stored overrides (affects: dump)"},
		},
		Examples: []string{
			"ripc config dump",
			"ripc config dump --scope my-app",
			"ripc config dump --zero",
			"ripc config dump --runtime",
			"ripc config set --scope my-app server.port 8080",
			"ripc config list --scope my-app",
			"ripc config rollback --scope my-app 3",
		},
	}
)

func printConfigUsage(w io.Writer) {
	commandConfig.Print(w, "ripc", "config")
}

func printConfigSetUsage(w io.Writer) {
	help := CommandHelp{
		Usage:       "ripc config set [options] <path> <value>",
		Description: "Sets a configuration value at a specified path.",
		Options: map[string]Option{
			"scope":  commandConfig.Options["scope"],
			"format": commandConfig.Options["format"],
			"desc":   commandConfig.Options["desc"],
		},
		Examples: []string{
			`ripc config set server.host localhost`,
			`ripc config set --scope webapp features.beta true --desc "Enable beta feature"`,
		},
	}
	help.Print(w, "ripc", "config", "set")
}

func handleConfigCommand(secureStore config.SecureStore, dbPool *sqlitex.Pool, commandArgs []string, ui UI) error {
	if len(commandArgs) < 1 {
		printConfigUsage(ui.Err)
		return fmt.Errorf("config requires a subcommand")
	}

	// Check for "help" subcommand
	if commandArgs[0] == "help" {
		if len(commandArgs) < 2 {
			printConfigUsage(ui.Out)
			return nil // Successful exit for general help
		}
		subcommandToHelp := commandArgs[1]
		switch subcommandToHelp {
		case "set":
			printConfigSetUsage(ui.Out)
		case "scaffold":
			printScaffoldUsage(ui.Out)
		// Add cases for other subcommands here as they get their own usage functions
		default:
			// For any other subcommand, show the main config usage.
			// This is helpful if they don't have a dedicated help page yet.
			printConfigUsage(ui.Out)
		}
		return nil // Successful exit for help display
	}

	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "set":
		scope, format, desc, path, value, remainingArgs, err := parseSetArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleSetCommand(secureStore, scope, format, desc, append([]string{path, value}, remainingArgs...), ui)
	case "scopes":
		err := parseScopesArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleScopesCommand(dbPool, ui)
	case "list":
		scope, err := parseListArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleListCommand(dbPool, scope, ui)
	case "paths":
		scope, filter, err := parsePathsArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handlePathsCommand(secureStore, scope, filter, ui)
	case "dump":
		scope, zero, runtime, err := parseDumpArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleDumpCommand(secureStore, scope, zero, runtime, ui)
	case "diff":
		scope, generation, err := parseDiffArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleDiffCommand(secureStore, scope, generation, ui)
	case "rollback":
		scope, generation, err := parseRollbackArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleRollbackCommand(secureStore, scope, generation, ui)
	case "save":
		scope, format, desc, filename, err := parseSaveArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleSaveCommand(secureStore, scope, format, desc, filename, ui)
	case "scaffold":
		scope, desc, scaffoldType, key, err := parseScaffoldArgs(subcommandArgs)
		if err != nil {
			printScaffoldUsage(ui.Err)
			return err
		}
		return handleScaffoldCommand(secureStore, scope, desc, scaffoldType, key, ui)
	case "get":
		scope, filter, err := parseGetArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleGetCommand(secureStore, scope, filter, ui)
	case "migrate":
		err := parseMigrateArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleMigrateCommand(secureStore, ui)
	default:
		printConfigUsage(ui.Err)
		return fmt.Errorf("unknown config subcommand: %s", subcommand)
	}
}
