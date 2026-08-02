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
			printConfigScaffoldUsage(ui.Out)
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
		opts, err := parseConfigSetArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleConfigSetCommand(secureStore, opts, ui)
	case "scopes":
		err := parseConfigScopesArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleConfigScopesCommand(dbPool, ui)
	case "list":
		opts, err := parseConfigListArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleConfigListCommand(dbPool, opts, ui)
	case "paths":
		opts, err := parseConfigPathsArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleConfigPathsCommand(secureStore, opts, ui)
	case "dump":
		opts, err := parseConfigDumpArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleConfigDumpCommand(secureStore, opts, ui)
	case "diff":
		opts, err := parseConfigDiffArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleConfigDiffCommand(secureStore, opts, ui)
	case "rollback":
		opts, err := parseConfigRollbackArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleConfigRollbackCommand(secureStore, opts, ui)
	case "save":
		opts, err := parseConfigSaveArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleConfigSaveCommand(secureStore, opts, ui)
	case "scaffold":
		opts, err := parseConfigScaffoldArgs(subcommandArgs)
		if err != nil {
			printConfigScaffoldUsage(ui.Err)
			return err
		}
		return handleConfigScaffoldCommand(secureStore, opts, ui)
	case "get":
		opts, err := parseConfigGetArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleConfigGetCommand(secureStore, opts, ui)
	case "migrate":
		err := parseConfigMigrateArgs(subcommandArgs)
		if err != nil {
			printConfigUsage(ui.Err)
			return err
		}
		return handleConfigMigrateCommand(secureStore, ui)
	default:
		printConfigUsage(ui.Err)
		return fmt.Errorf("unknown config subcommand: %s", subcommand)
	}
}
