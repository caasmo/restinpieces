package main


import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/caasmo/restinpieces/config"
	"zombiezen.com/go/sqlite/sqlitex"
)

var (
	ErrUnknownSubcommand = errors.New("unknown config subcommand")

	// commandConfig is the single source of truth for the 'ripc config' command's definition and help text.
	commandConfig = CommandHelp{
		Usage:       fmt.Sprintf("%s config <subcommand> [options]", os.Args[0]),
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
					{"init", "Initialize the configuration with default values"},
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
			"scope":  {DefaultValue: config.ScopeApplication, Usage: "Scope for the configuration (affects: set, get, paths, dump, diff, rollback, save)"},
			"format": {DefaultValue: "toml", Usage: "Format of the configuration file (affects: set, save)"},
			"desc":   {Usage: "Optional description for this configuration version (affects: set, save)"},
		},
		Examples: []string{
			"ripc config set --scope my-app server.port 8080",
			"ripc config list --scope my-app",
			"ripc config rollback --scope my-app 3",
		},
	}
)

func printConfigUsage() {
	commandConfig.Print(os.Stderr, "ripc", "config")
}

func printConfigSetUsage() {
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
	help.Print(os.Stderr, "ripc", "config", "set")
}

func handleConfigCommand(secureStore config.SecureStore, dbPool *sqlitex.Pool, commandArgs []string) {
	if len(commandArgs) < 1 {
		printConfigUsage()
		os.Exit(1)
	}

	// Check for "help" subcommand
	if commandArgs[0] == "help" {
		if len(commandArgs) < 2 {
			printConfigUsage()
			os.Exit(0) // Successful exit for general help
		}
		subcommandToHelp := commandArgs[1]
		switch subcommandToHelp {
		case "set":
			printConfigSetUsage()
		// Add cases for other subcommands here as they get their own usage functions
		default:
			// For any other subcommand, show the main config usage.
			// This is helpful if they don't have a dedicated help page yet.
			printConfigUsage()
		}
		os.Exit(0) // Successful exit for help display
	}

	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "set":
		scope, format, desc, path, value, remainingArgs, err := parseSetArgs(subcommandArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			printConfigUsage()
			os.Exit(1)
		}
		handleSetCommand(secureStore, scope, format, desc, append([]string{path, value}, remainingArgs...))
	case "scopes":
		if err := parseScopesArgs(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			printConfigUsage()
			os.Exit(1)
		}
		handleScopesCommand(dbPool)
	case "list":
		scope, err := parseListArgs(subcommandArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			printConfigUsage()
			os.Exit(1)
		}
		handleListCommand(dbPool, scope)
	case "paths":
		scope, filter, err := parsePathsArgs(subcommandArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			printConfigUsage()
			os.Exit(1)
		}
		handlePathsCommand(secureStore, scope, filter)
	case "dump":
		scope, err := parseDumpArgs(subcommandArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			printConfigUsage()
			os.Exit(1)
		}
		handleDumpCommand(secureStore, scope)
	case "diff":
		scope, generation, err := parseDiffArgs(subcommandArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			printConfigUsage()
			os.Exit(1)
		}
		handleDiffCommand(secureStore, scope, generation)
	case "rollback":
		scope, generation, err := parseRollbackArgs(subcommandArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			printConfigUsage()
			os.Exit(1)
		}
		handleRollbackCommand(secureStore, scope, generation)
	case "save":
		scope, format, desc, filename, err := parseSaveArgs(subcommandArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			printConfigUsage()
			os.Exit(1)
		}
		handleSaveCommand(secureStore, scope, format, desc, filename)
	case "get":
		scope, filter, err := parseGetArgs(subcommandArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			printConfigUsage()
			os.Exit(1)
		}
		handleGetCommand(secureStore, scope, filter)
	case "init":
		if err := parseInitArgs(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			printConfigUsage()
			os.Exit(1)
		}
		handleInitCommand(secureStore)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown config subcommand: %s\n", subcommand)
		printConfigUsage()
		os.Exit(1)
	}
}


// Individual parsing functions for each subcommand

func parseSetArgs(args []string) (scope, format, desc, path, value string, remainingArgs []string, err error) {
	setCmd := flag.NewFlagSet("set", flag.ContinueOnError)
	setCmd.SetOutput(io.Discard) // Output not needed for parsing
	scopeOpt := commandConfig.Options["scope"]
	formatOpt := commandConfig.Options["format"]
	descOpt := commandConfig.Options["desc"]
	setScope := setCmd.String("scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	formatFlag := setCmd.String("format", formatOpt.DefaultValue, formatOpt.Usage)
	descFlag := setCmd.String("desc", descOpt.DefaultValue, descOpt.Usage)

	if err := setCmd.Parse(args); err != nil {
		return "", "", "", "", "", nil, fmt.Errorf("parsing set flags: %w: %v", ErrInvalidFlag, err)
	}
	if setCmd.NArg() < 2 {
		return "", "", "", "", "", nil, fmt.Errorf("'set' requires path and value arguments: %w", ErrMissingArgument)
	}
	return *setScope, *formatFlag, *descFlag, setCmd.Arg(0), setCmd.Arg(1), setCmd.Args()[2:], nil
}

func parseScopesArgs(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("'scopes' command does not take any arguments: %w", ErrTooManyArguments)
	}
	return nil
}

func parseListArgs(args []string) (scope string, err error) {
	if len(args) > 1 {
		return "", fmt.Errorf("'list' command takes at most one scope argument: %w", ErrTooManyArguments)
	}
	if len(args) > 0 {
		return args[0], nil
	}
	return "", nil
}

func parsePathsArgs(args []string) (scope, filter string, err error) {
	pathsCmd := flag.NewFlagSet("paths", flag.ContinueOnError)
	pathsCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]
	pathsScope := pathsCmd.String("scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	if err := pathsCmd.Parse(args); err != nil {
		return "", "", fmt.Errorf("parsing paths flags: %w: %v", ErrInvalidFlag, err)
	}
	filter = ""
	if pathsCmd.NArg() > 0 {
		filter = pathsCmd.Arg(0)
	}
	if pathsCmd.NArg() > 1 {
		return "", "", fmt.Errorf("'paths' command takes at most one filter argument: %w", ErrTooManyArguments)
	}
	return *pathsScope, filter, nil
}

func parseDumpArgs(args []string) (scope string, err error) {
	dumpCmd := flag.NewFlagSet("dump", flag.ContinueOnError)
	dumpCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]
	dumpScope := dumpCmd.String("scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	if err := dumpCmd.Parse(args); err != nil {
		return "", fmt.Errorf("parsing dump flags: %w: %v", ErrInvalidFlag, err)
	}
	if dumpCmd.NArg() > 0 {
		return "", fmt.Errorf("'dump' command does not take any arguments: %w", ErrTooManyArguments)
	}
	return *dumpScope, nil
}

func parseDiffArgs(args []string) (scope string, generation int, err error) {
	diffCmd := flag.NewFlagSet("diff", flag.ContinueOnError)
	diffCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]
	diffScope := diffCmd.String("scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	if err := diffCmd.Parse(args); err != nil {
		return "", 0, fmt.Errorf("parsing diff flags: %w: %v", ErrInvalidFlag, err)
	}
	if diffCmd.NArg() < 1 {
		return "", 0, fmt.Errorf("'diff' requires generation number argument: %w", ErrMissingArgument)
	}
	if diffCmd.NArg() > 1 {
		return "", 0, fmt.Errorf("'diff' command takes at most one generation argument: %w", ErrTooManyArguments)
	}
	gen, err := strconv.Atoi(diffCmd.Arg(0))
	if err != nil {
		return "", 0, fmt.Errorf("generation must be a number: %w", ErrNotANumber)
	}
	return *diffScope, gen, nil
}

func parseRollbackArgs(args []string) (scope string, generation int, err error) {
	rollbackCmd := flag.NewFlagSet("rollback", flag.ContinueOnError)
	rollbackCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]
	rollbackScope := rollbackCmd.String("scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	if err := rollbackCmd.Parse(args); err != nil {
		return "", 0, fmt.Errorf("parsing rollback flags: %w: %v", ErrInvalidFlag, err)
	}
	if rollbackCmd.NArg() < 1 {
		return "", 0, fmt.Errorf("'rollback' requires generation number argument: %w", ErrMissingArgument)
	}
	if rollbackCmd.NArg() > 1 {
		return "", 0, fmt.Errorf("'rollback' command takes at most one generation argument: %w", ErrTooManyArguments)
	}
	gen, err := strconv.Atoi(rollbackCmd.Arg(0))
	if err != nil {
		return "", 0, fmt.Errorf("generation must be a number: %w", ErrNotANumber)
	}
	return *rollbackScope, gen, nil
}

func parseSaveArgs(args []string) (scope, format, desc, filename string, err error) {
	saveCmd := flag.NewFlagSet("save", flag.ContinueOnError)
	saveCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]
	formatOpt := commandConfig.Options["format"]
	descOpt := commandConfig.Options["desc"]
	saveScope := saveCmd.String("scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	formatFlag := saveCmd.String("format", "", formatOpt.Usage) // Corrected default value
	descFlag := saveCmd.String("desc", descOpt.DefaultValue, descOpt.Usage)

	if err := saveCmd.Parse(args); err != nil {
		return "", "", "", "", fmt.Errorf("parsing save flags: %w: %v", ErrInvalidFlag, err)
	}
	if saveCmd.NArg() < 1 {
		return "", "", "", "", fmt.Errorf("'save' requires filename argument: %w", ErrMissingArgument)
	}
	if saveCmd.NArg() > 1 {
		return "", "", "", "", fmt.Errorf("'save' command takes at most one filename argument: %w", ErrTooManyArguments)
	}
	return *saveScope, *formatFlag, *descFlag, saveCmd.Arg(0), nil
}

func parseGetArgs(args []string) (scope, filter string, err error) {
	getCmd := flag.NewFlagSet("get", flag.ContinueOnError)
	getCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]
	getScope := getCmd.String("scope", scopeOpt.DefaultValue, scopeOpt.Usage)

	if err := getCmd.Parse(args); err != nil {
		return "", "", fmt.Errorf("parsing get flags: %w: %v", ErrInvalidFlag, err)
	}
	filter = ""
	if getCmd.NArg() > 0 {
		filter = getCmd.Arg(0)
	}
	if getCmd.NArg() > 1 {
		return "", "", fmt.Errorf("'get' command takes at most one filter argument: %w", ErrTooManyArguments)
	}
	return *getScope, filter, nil
}

func parseInitArgs(args []string) error {
	initCmd := flag.NewFlagSet("init", flag.ContinueOnError)
	initCmd.SetOutput(io.Discard)
	if err := initCmd.Parse(args); err != nil {
		return fmt.Errorf("parsing init flags: %w: %v", ErrInvalidFlag, err)
	}
	if initCmd.NArg() > 0 {
		return fmt.Errorf("'init' does not take any arguments: %w", ErrTooManyArguments)
	}
	return nil
}

