package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/caasmo/restinpieces/config"
	"zombiezen.com/go/sqlite/sqlitex"
)

func printConfigUsage() {
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

	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "set":
		setCmd := flag.NewFlagSet("set", flag.ContinueOnError)
		setScope := setCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		formatFlag := setCmd.String("format", "toml", "Format of the configuration file (e.g., 'toml', 'json')")
		descFlag := setCmd.String("desc", "", "Optional description for this configuration version")
		if err := setCmd.Parse(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing set flags: %v\n", err)
			setCmd.Usage()
			os.Exit(1)
		}
		if setCmd.NArg() < 2 {
			fmt.Fprintf(os.Stderr, "Error: 'set' requires path and value arguments\n")
			setCmd.Usage()
			os.Exit(1)
		}
		handleSetCommand(secureStore, *setScope, *formatFlag, *descFlag, setCmd.Args())
	case "scopes":
		if len(subcommandArgs) > 0 {
			fmt.Fprintf(os.Stderr, "Error: 'scopes' command does not take any arguments\n")
			printConfigUsage()
			os.Exit(1)
		}
		handleScopesCommand(dbPool)
	case "list":
		scopeToList := ""
		if len(subcommandArgs) > 0 {
			scopeToList = subcommandArgs[0]
			if len(subcommandArgs) > 1 {
				fmt.Fprintf(os.Stderr, "Error: 'list' command takes at most one scope argument\n")
				printConfigUsage()
				os.Exit(1)
			}
		}
		handleListCommand(dbPool, scopeToList)
	case "paths":
		pathsCmd := flag.NewFlagSet("paths", flag.ContinueOnError)
		pathsScope := pathsCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := pathsCmd.Parse(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing paths flags: %v\n", err)
			pathsCmd.Usage()
			os.Exit(1)
		}
		filter := ""
		if pathsCmd.NArg() > 0 {
			filter = pathsCmd.Arg(0)
		}
		handlePathsCommand(secureStore, *pathsScope, filter)
	case "dump":
		dumpCmd := flag.NewFlagSet("dump", flag.ContinueOnError)
		dumpScope := dumpCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := dumpCmd.Parse(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing dump flags: %v\n", err)
			dumpCmd.Usage()
			os.Exit(1)
		}
		handleDumpCommand(secureStore, *dumpScope)
	case "diff":
		diffCmd := flag.NewFlagSet("diff", flag.ContinueOnError)
		diffScope := diffCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := diffCmd.Parse(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing diff flags: %v\n", err)
			diffCmd.Usage()
			os.Exit(1)
		}
		if diffCmd.NArg() < 1 {
			fmt.Fprintf(os.Stderr, "Error: 'diff' requires generation number argument\n")
			diffCmd.Usage()
			os.Exit(1)
		}
		gen, err := strconv.Atoi(diffCmd.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: generation must be a number\n")
			os.Exit(1)
		}
		handleDiffCommand(secureStore, *diffScope, gen)
	case "rollback":
		rollbackCmd := flag.NewFlagSet("rollback", flag.ContinueOnError)
		rollbackScope := rollbackCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := rollbackCmd.Parse(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing rollback flags: %v\n", err)
			rollbackCmd.Usage()
			os.Exit(1)
		}
		if rollbackCmd.NArg() < 1 {
			fmt.Fprintf(os.Stderr, "Error: 'rollback' requires generation number argument\n")
			rollbackCmd.Usage()
			os.Exit(1)
		}
		gen, err := strconv.Atoi(rollbackCmd.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: generation must be a number\n")
			os.Exit(1)
		}
		handleRollbackCommand(secureStore, *rollbackScope, gen)
	case "save":
		saveCmd := flag.NewFlagSet("save", flag.ContinueOnError)
		saveScope := saveCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := saveCmd.Parse(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing save flags: %v\n", err)
			saveCmd.Usage()
			os.Exit(1)
		}
		if saveCmd.NArg() < 1 {
			fmt.Fprintf(os.Stderr, "Error: 'save' requires filename argument\n")
			saveCmd.Usage()
			os.Exit(1)
		}
		handleSaveCommand(secureStore, *saveScope, saveCmd.Arg(0))
	case "get":
		getCmd := flag.NewFlagSet("get", flag.ContinueOnError)
		getScope := getCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := getCmd.Parse(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing get flags: %v\n", err)
			getCmd.Usage()
			os.Exit(1)
		}
		filter := ""
		if getCmd.NArg() > 0 {
			filter = getCmd.Arg(0)
		}
		handleGetCommand(secureStore, *getScope, filter)
	case "init":
		initCmd := flag.NewFlagSet("init", flag.ContinueOnError)
		initScope := initCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		if err := initCmd.Parse(subcommandArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing init flags: %v\n", err)
			initCmd.Usage()
			os.Exit(1)
		}
		if initCmd.NArg() > 0 {
			fmt.Fprintf(os.Stderr, "Error: 'init' does not take any arguments\n")
			initCmd.Usage()
			os.Exit(1)
		}
		handleInitCommand(secureStore, *initScope)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown config subcommand: %s\n", subcommand)
		printConfigUsage()
		os.Exit(1)
	}
}
