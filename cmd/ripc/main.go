package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
)

var (
	// main application errors
	ErrMissingFlag       = errors.New("missing required global flag")
	ErrMissingCommand    = errors.New("missing command")
	ErrUnknownCommand    = errors.New("unknown command")
	ErrDBNotFound        = errors.New("database file not found")
	ErrDBAlreadyExists   = errors.New("database file already exists")
	ErrCreateSecureStore = errors.New("failed to instantiate secure store")
)

// prog is the invoked program path, the single source of truth for the
// program name in usage output.
var prog = os.Args[0]

// commandOptions is the single source of truth for the flags shared by the
// flattened configuration commands. Each command's Spec and flag registration
// read from it via Spec.Opt. It is a Spec (rather than a []OptSpec) so the
// lookup reuses the framework's Spec.Opt helper.
var commandOptions = Spec{
	Options: []OptSpec{
		{Name: "scope", Meta: "string", DefaultValue: config.ScopeApplication, Usage: "Scope for the configuration"},
		{Name: "format", Meta: "string", DefaultValue: "toml", Usage: "Format of the configuration file"},
		{Name: "desc", Meta: "string", Usage: "Optional description for this configuration version"},
		{Name: "zero", Usage: "Output stored overrides on top of zero values"},
		{Name: "runtime", Usage: "Output defaults merged with stored overrides"},
	},
}

// UI contains the output streams for the application.
// Used for injecting buffers during testing.
type UI struct {
	Out io.Writer
	Err io.Writer
}

func fprintErr(w io.Writer, err error) {
	_, _ = fmt.Fprintf(w, "Error: %v\n", err)
}

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fprintErr(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	ui := UI{Out: os.Stdout, Err: output}

	// We need a new flag set for each run
	fs := flag.NewFlagSet("ripc", flag.ContinueOnError)
	fs.SetOutput(output)

	// Global flags
	ageIdentityPathFlag := fs.String("agekey", os.Getenv("RIPC_AGE_KEY_PATH"), "Path to the age identity file (private key 'AGE-SECRET-KEY-1...')")
	dbPathFlag := fs.String("dbpath", os.Getenv("RIPC_DB"), "Path to the SQLite database file")

	fs.Usage = func() {
		help := Spec{
			Usage:       "[global options] <command> [command-specific options]",
			Description: "A tool for managing the Rip application, including configuration, authentication, and jobs.",
			GlobalOptions: []OptSpec{
				{Name: "agekey", Meta: "string", Usage: "Path to the age identity file (private key 'AGE-SECRET-KEY-1...')"},
				{Name: "dbpath", Meta: "string", Usage: "Path to the SQLite database file"},
			},
			Subcommands: []SubcommandGroup{
				{
					Title: "Reading Configuration",
					Subcommands: []Subcommand{
						{"get", "Get configuration values by path"},
						{"paths", "List all keys in the configuration"},
						{"dump", "Dump the configuration"},
						{"scopes", "List all configuration scopes"},
					},
				},
				{
					Title: "Modifying Configuration",
					Subcommands: []Subcommand{
						{"set", "Set a configuration value"},
						{"save", "Save file contents to the configuration"},
						{"scaffold", "Scaffold a configuration entry with defaults"},
						{"migrate", "Migrate configuration to current framework version"},
					},
				},
				{
					Title: "Version Control",
					Subcommands: []Subcommand{
						{"list", "List configuration versions"},
						{"diff", "Compare configuration versions"},
						{"rollback", "Restore a previous configuration version"},
					},
				},
				{
					Title: "Management",
					Subcommands: []Subcommand{
						{"app", "Manage application lifecycle (e.g., creating the database)"},
						{"job", "Manage background jobs"},
						{"log", "Manage the log database"},
						{"help", "Show help for a specific command"},
					},
				},
			},
			Examples: []string{
				"ripc app create",
				"ripc set server.port 8080",
			},
		}
		help.Print(output, prog)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", ErrInvalidFlag, err)
	}

	if *ageIdentityPathFlag == "" {
		fs.Usage()
		return fmt.Errorf("%w: -agekey flag or RIPC_AGE_KEY_PATH env must be provided", ErrMissingFlag)
	}
	if *dbPathFlag == "" {
		fs.Usage()
		return fmt.Errorf("%w: -dbpath flag or RIPC_DB env must be provided", ErrMissingFlag)
	}

	cmdArgs := fs.Args()
	if len(cmdArgs) < 1 {
		fs.Usage()
		return nil // Successfully show usage and exit.
	}

	command := cmdArgs[0]
	commandArgs := cmdArgs[1:]

	isAppCreate := command == "app" && len(commandArgs) > 0 && commandArgs[0] == "create"
	if !isAppCreate {
		if _, err := os.Stat(*dbPathFlag); os.IsNotExist(err) {
			// Not using the writeUsage helper here as this is a specific error message, not part of the general usage.
			_, _ = fmt.Fprintf(output, "Error: database file not found: %s\n", *dbPathFlag)
			_, _ = fmt.Fprintf(output, "Please create it first using 'ripc app create'.\n")
			return ErrDBNotFound
		}
	} else { // for app create, the database must NOT exist
		if _, err := os.Stat(*dbPathFlag); err == nil {
			_, _ = fmt.Fprintf(output, "Error: database file already exists: %s\n", *dbPathFlag)
			return ErrDBAlreadyExists
		}
	}

	db, err := newAppDb(*dbPathFlag)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			_, _ = fmt.Fprintf(output, "Error: error closing database pool: %v\n", err)
		}
	}()

	secureStore, err := config.NewSecureStoreAge(db, *ageIdentityPathFlag)
	if err != nil {
		return fmt.Errorf("%w (age, age_key_path: %s): %v", ErrCreateSecureStore, *ageIdentityPathFlag, err)
	}

	switch command {
	case "app":
		return handleAppCommand(secureStore, db, *dbPathFlag, commandArgs, ui)
	case "get":
		return handleGetCommand(secureStore, commandArgs, ui)
	case "paths":
		return handlePathsCommand(secureStore, commandArgs, ui)
	case "dump":
		return handleDumpCommand(secureStore, commandArgs, ui)
	case "scopes":
		return handleScopesCommand(db, commandArgs, ui)
	case "set":
		return handleSetCommand(secureStore, commandArgs, ui)
	case "save":
		return handleSaveCommand(secureStore, commandArgs, ui)
	case "scaffold":
		return handleScaffoldCommand(secureStore, commandArgs, ui)
	case "migrate":
		return handleMigrateCommand(secureStore, commandArgs, ui)
	case "list":
		return handleListCommand(db, commandArgs, ui)
	case "diff":
		return handleDiffCommand(secureStore, commandArgs, ui)
	case "rollback":
		return handleRollbackCommand(secureStore, commandArgs, ui)
	case "job":
		return handleJobCommand(db.Db, commandArgs, ui)
	case "log":
		return handleLogCommand(secureStore, *dbPathFlag, commandArgs, ui)
	case "help":
		return handleHelpCommand(commandArgs, fs.Usage, ui)
	default:
		fs.Usage()
		return fmt.Errorf("%w: %s", ErrUnknownCommand, command)
	}
}
