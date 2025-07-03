package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/caasmo/restinpieces/config"
)

func handleConfigCommand(secureStore *config.SecureStore, dbPool *sql.DB, commandArgs []string) {
	if len(commandArgs) < 1 {
		fmt.Fprintln(os.Stderr, "Error: config command requires a subcommand (set, scopes, list, paths, dump, diff, rollback, save, get, init)")
		os.Exit(1)
	}

	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "set":
		setCmd := flag.NewFlagSet("set", flag.ExitOnError)
		setScope := setCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		formatFlag := setCmd.String("format", "toml", "Format of the configuration file (e.g., 'toml', 'json')")
		descFlag := setCmd.String("desc", "", "Optional description for this configuration version")
		setCmd.Parse(subcommandArgs)
		if setCmd.NArg() < 2 {
			fmt.Fprintf(os.Stderr, "Error: 'set' requires path and value arguments\n")
			setCmd.Usage()
			os.Exit(1)
		}
		handleSetCommand(secureStore, *setScope, *formatFlag, *descFlag, setCmd.Args())
	case "scopes":
		if len(subcommandArgs) > 0 {
			fmt.Fprintf(os.Stderr, "Error: 'scopes' command does not take any arguments\n")
			os.Exit(1)
		}
		handleScopesCommand(dbPool)
	case "list":
		scopeToList := ""
		if len(subcommandArgs) > 0 {
			scopeToList = subcommandArgs[0]
			if len(subcommandArgs) > 1 {
				fmt.Fprintf(os.Stderr, "Error: 'list' command takes at most one scope argument\n")
				os.Exit(1)
			}
		}
		handleListCommand(dbPool, scopeToList)
	case "paths":
		pathsCmd := flag.NewFlagSet("paths", flag.ExitOnError)
		pathsScope := pathsCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		pathsCmd.Parse(subcommandArgs)
		filter := ""
		if pathsCmd.NArg() > 0 {
			filter = pathsCmd.Arg(0)
		}
		handlePathsCommand(secureStore, *pathsScope, filter)
	case "dump":
		dumpCmd := flag.NewFlagSet("dump", flag.ExitOnError)
		dumpScope := dumpCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		dumpCmd.Parse(subcommandArgs)
		handleDumpCommand(secureStore, *dumpScope)
	case "diff":
		diffCmd := flag.NewFlagSet("diff", flag.ExitOnError)
		diffScope := diffCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		diffCmd.Parse(subcommandArgs)
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
		rollbackCmd := flag.NewFlagSet("rollback", flag.ExitOnError)
		rollbackScope := rollbackCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		rollbackCmd.Parse(subcommandArgs)
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
		saveCmd := flag.NewFlagSet("save", flag.ExitOnError)
		saveScope := saveCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		saveCmd.Parse(subcommandArgs)
		if saveCmd.NArg() < 1 {
			fmt.Fprintf(os.Stderr, "Error: 'save' requires filename argument\n")
			saveCmd.Usage()
			os.Exit(1)
		}
		handleSaveCommand(secureStore, *saveScope, saveCmd.Arg(0))
	case "get":
		getCmd := flag.NewFlagSet("get", flag.ExitOnError)
		getScope := getCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		getCmd.Parse(subcommandArgs)
		filter := ""
		if getCmd.NArg() > 0 {
			filter = getCmd.Arg(0)
		}
		handleGetCommand(secureStore, *getScope, filter)
	case "init":
		initCmd := flag.NewFlagSet("init", flag.ExitOnError)
		initScope := initCmd.String("scope", config.ScopeApplication, "Scope for the configuration")
		initCmd.Parse(subcommandArgs)
		if initCmd.NArg() > 0 {
			fmt.Fprintf(os.Stderr, "Error: 'init' does not take any arguments\n")
			initCmd.Usage()
			os.Exit(1)
		}
		handleInitCommand(secureStore, *initScope)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown config subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}