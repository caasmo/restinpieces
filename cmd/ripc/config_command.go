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
)

func printConfigUsage() {
	help := CommandHelp{
		Usage:       "ripc config <subcommand> [options]",
		Description: "Manages the application's secure configuration.",
		Subcommands: []SubcommandGroup{
			{
				Title: "Configuration Management",
				Subcommands: []Subcommand{
					{"set", "Set a configuration value"},
					{"get", "Get configuration values by path"},
					{"save", "Save file contents to the configuration"},
					{"init", "Initialize the configuration with default values"},
				},
			},
			{
				Title: "History and Versioning",
				Subcommands: []Subcommand{
					{"list", "List configuration versions"},
					{"scopes", "List all configuration scopes"},
					{"diff", "Compare configuration versions"},
					{"rollback", "Restore a previous configuration version"},
				},
			},
			{
				Title: "Inspection",
				Subcommands: []Subcommand{
					{"paths", "List all keys in the configuration"},
					{"dump", "Dump the full configuration"},
				},
			},
		},
		Examples: []string{
			"ripc config set --scope my-app server.port 8080",
			"ripc config list --scope my-app",
			"ripc config rollback --scope my-app 3",
		},
	}
	help.Print(os.Stderr, "ripc", "config")
}

func printConfigSetUsage() {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.String("scope", config.ScopeApplication, "Scope for the configuration")
	fs.String("format", "toml", "Format of the configuration file (e.g., 'toml', 'json')")
	fs.String("desc", "", "Optional description for this configuration version")

	help := CommandHelp{
		Usage:       "ripc config set [options] <path> <value>",
		Description: "Sets a configuration value at a specified path.",
		Options:     fs,
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

	subcommand, subcommandArgs, err := parseConfigSubcommand(commandArgs, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		// Potentially print usage for the specific subcommand if flags were involved
		printConfigUsage()
		os.Exit(1)
	}

	switch subcommand {
	case "set":
		handleSetCommand(secureStore, subcommandArgs[0], subcommandArgs[1], subcommandArgs[2], subcommandArgs[3:])
	case "scopes":
		handleScopesCommand(dbPool)
	case "list":
		scopeToList := ""
		if len(subcommandArgs) > 0 {
			scopeToList = subcommandArgs[0]
		}
		handleListCommand(dbPool, scopeToList)
	case "paths":
		handlePathsCommand(secureStore, subcommandArgs[0], subcommandArgs[1])
	case "dump":
		handleDumpCommand(secureStore, subcommandArgs[0])
	case "diff":
		gen, _ := strconv.Atoi(subcommandArgs[1])
		handleDiffCommand(secureStore, subcommandArgs[0], gen)
	case "rollback":
		gen, _ := strconv.Atoi(subcommandArgs[1])
		handleRollbackCommand(secureStore, subcommandArgs[0], gen)
	case "save":
		handleSaveCommand(secureStore, subcommandArgs[0], subcommandArgs[1], subcommandArgs[2], subcommandArgs[3])
	case "get":
		handleGetCommand(secureStore, subcommandArgs[0], subcommandArgs[1])
	case "init":
		handleInitCommand(secureStore)
	default:
		// This case should ideally not be reached if parseConfigSubcommand is correct
		fmt.Fprintf(os.Stderr, "Error: unknown config subcommand: %s\n", subcommand)
		printConfigUsage()
		os.Exit(1)
	}
}

func parseConfigSubcommand(commandArgs []string, output io.Writer) (string, []string, error) {
	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "set":
		setCmd := flag.NewFlagSet("set", flag.ContinueOnError)
		setCmd.SetOutput(output)
		setScope := setCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		formatFlag := setCmd.String("format", "toml", "Format of the configuration file (e.g., 'toml', 'json')")
		descFlag := setCmd.String("desc", "", "Optional description for this configuration version")
		if err := setCmd.Parse(subcommandArgs); err != nil {
			return "", nil, fmt.Errorf("parsing set flags: %w: %v", ErrInvalidFlag, err)
		}
		if setCmd.NArg() < 2 {
			return "", nil, fmt.Errorf("'set' requires path and value arguments: %w", ErrMissingArgument)
		}
		// Return flags and args in a specific order
		return subcommand, append([]string{*setScope, *formatFlag, *descFlag}, setCmd.Args()...), nil
	case "scopes":
		if len(subcommandArgs) > 0 {
			return "", nil, fmt.Errorf("'scopes' command does not take any arguments: %w", ErrTooManyArguments)
		}
		return subcommand, nil, nil
	case "list":
		if len(subcommandArgs) > 1 {
			return "", nil, fmt.Errorf("'list' command takes at most one scope argument: %w", ErrTooManyArguments)
		}
		return subcommand, subcommandArgs, nil
	case "paths":
		pathsCmd := flag.NewFlagSet("paths", flag.ContinueOnError)
		pathsCmd.SetOutput(output)
		pathsScope := pathsCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := pathsCmd.Parse(subcommandArgs); err != nil {
			return "", nil, fmt.Errorf("parsing paths flags: %w: %v", ErrInvalidFlag, err)
		}
		filter := ""
		if pathsCmd.NArg() > 0 {
			filter = pathsCmd.Arg(0)
		}
		if pathsCmd.NArg() > 1 {
			return "", nil, fmt.Errorf("'paths' command takes at most one filter argument: %w", ErrTooManyArguments)
		}
		return subcommand, []string{*pathsScope, filter}, nil
	case "dump":
		dumpCmd := flag.NewFlagSet("dump", flag.ContinueOnError)
		dumpCmd.SetOutput(output)
		dumpScope := dumpCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := dumpCmd.Parse(subcommandArgs); err != nil {
			return "", nil, fmt.Errorf("parsing dump flags: %w: %v", ErrInvalidFlag, err)
		}
		if dumpCmd.NArg() > 0 {
			return "", nil, fmt.Errorf("'dump' command does not take any arguments: %w", ErrTooManyArguments)
		}
		return subcommand, []string{*dumpScope}, nil
	case "diff":
		diffCmd := flag.NewFlagSet("diff", flag.ContinueOnError)
		diffCmd.SetOutput(output)
		diffScope := diffCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := diffCmd.Parse(subcommandArgs); err != nil {
			return "", nil, fmt.Errorf("parsing diff flags: %w: %v", ErrInvalidFlag, err)
		}
		if diffCmd.NArg() < 1 {
			return "", nil, fmt.Errorf("'diff' requires generation number argument: %w", ErrMissingArgument)
		}
		if diffCmd.NArg() > 1 {
			return "", nil, fmt.Errorf("'diff' command takes at most one generation argument: %w", ErrTooManyArguments)
		}
		_, err := strconv.Atoi(diffCmd.Arg(0))
		if err != nil {
			return "", nil, fmt.Errorf("generation must be a number: %w", ErrNotANumber)
		}
		return subcommand, []string{*diffScope, diffCmd.Arg(0)}, nil
	case "rollback":
		rollbackCmd := flag.NewFlagSet("rollback", flag.ContinueOnError)
		rollbackCmd.SetOutput(output)
		rollbackScope := rollbackCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := rollbackCmd.Parse(subcommandArgs); err != nil {
			return "", nil, fmt.Errorf("parsing rollback flags: %w: %v", ErrInvalidFlag, err)
		}
		if rollbackCmd.NArg() < 1 {
			return "", nil, fmt.Errorf("'rollback' requires generation number argument: %w", ErrMissingArgument)
		}
		if rollbackCmd.NArg() > 1 {
			return "", nil, fmt.Errorf("'rollback' command takes at most one generation argument: %w", ErrTooManyArguments)
		}
		_, err := strconv.Atoi(rollbackCmd.Arg(0))
		if err != nil {
			return "", nil, fmt.Errorf("generation must be a number: %w", ErrNotANumber)
		}
		return subcommand, []string{*rollbackScope, rollbackCmd.Arg(0)}, nil
	case "save":
		saveCmd := flag.NewFlagSet("save", flag.ContinueOnError)
		saveCmd.SetOutput(output)
		saveScope := saveCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		formatFlag := saveCmd.String("format", "", "Format of the configuration file (e.g., 'toml', 'json'). Auto-detected if omitted.")
		descFlag := saveCmd.String("desc", "", "Optional description for this configuration version")
		if err := saveCmd.Parse(subcommandArgs); err != nil {
			return "", nil, fmt.Errorf("parsing save flags: %w: %v", ErrInvalidFlag, err)
		}
		if saveCmd.NArg() < 1 {
			return "", nil, fmt.Errorf("'save' requires filename argument: %w", ErrMissingArgument)
		}
		if saveCmd.NArg() > 1 {
			return "", nil, fmt.Errorf("'save' command takes at most one filename argument: %w", ErrTooManyArguments)
		}
		return subcommand, append([]string{*saveScope, *formatFlag, *descFlag}, saveCmd.Args()...), nil
	case "get":
		getCmd := flag.NewFlagSet("get", flag.ContinueOnError)
		getCmd.SetOutput(output)
		getScope := getCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := getCmd.Parse(subcommandArgs); err != nil {
			return "", nil, fmt.Errorf("parsing get flags: %w: %v", ErrInvalidFlag, err)
		}
		filter := ""
		if getCmd.NArg() > 0 {
			filter = getCmd.Arg(0)
		}
		if getCmd.NArg() > 1 {
			return "", nil, fmt.Errorf("'get' command takes at most one filter argument: %w", ErrTooManyArguments)
		}
		return subcommand, []string{*getScope, filter}, nil
	case "init":
		initCmd := flag.NewFlagSet("init", flag.ContinueOnError)
		initCmd.SetOutput(output)
		if err := initCmd.Parse(subcommandArgs); err != nil {
			return "", nil, fmt.Errorf("parsing init flags: %w: %v", ErrInvalidFlag, err)
		}
		if initCmd.NArg() > 0 {
			return "", nil, fmt.Errorf("'init' does not take any arguments: %w", ErrTooManyArguments)
		}
		return subcommand, nil, nil
	default:
		return "", nil, fmt.Errorf("'%s': %w", subcommand, ErrUnknownSubcommand)
	}
}

