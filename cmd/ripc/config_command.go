package main


import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/caasmo/restinpieces/config"
	"zombiezen.com/go/sqlite/sqlitex"
)

var (
	ErrUnknownSubcommand = errors.New("unknown config subcommand")
)

func printConfigUsage() {
// ... (rest of the file is unchanged)

	fmt.Fprintf(os.Stderr, "Usage: %s config <subcommand> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Manages the application configuration.\n\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  set <path> <value>    Set a configuration value\n")
	fmt.Fprintf(os.Stderr, "  scopes                List all configuration scopes\n")
	fmt.Fprintf(os.Stderr, "  list [scope]          List configuration versions\n")
	fmt.Fprintf(os.Stderr, "  paths [filter]        List all keys in the configuration\n")
	fmt.Fprintf(os.Stderr, "  dump                  Dump the configuration\n")
	fmt.Fprintf(os.Stderr, "  diff <generation>     Compare configuration versions\n")
	fmt.Fprintf(os.Stderr, "  rollback <generation> Restore a previous configuration version\n")
	fmt.Fprintf(os.Stderr, "  save <file>           Save file contents to the configuration\n")
	fmt.Fprintf(os.Stderr, "  get [filter]          Get configuration values by path\n")
	fmt.Fprintf(os.Stderr, "  init                  Initialize the configuration with default values\n")
}

func handleConfigCommand(secureStore config.SecureStore, dbPool *sqlitex.Pool, commandArgs []string) {
	if len(commandArgs) < 1 {
		printConfigUsage()
		os.Exit(1)
	}

	subcommand, subcommandArgs, err := parseConfigSubcommand(commandArgs)
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

func parseConfigSubcommand(commandArgs []string) (string, []string, error) {
	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "set":
		setCmd := flag.NewFlagSet("set", flag.ContinueOnError)
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
		saveScope := saveCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		formatFlag := saveCmd.String("format", "toml", "Format of the configuration file (e.g., 'toml', 'json')")
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

