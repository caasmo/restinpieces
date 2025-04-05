package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/custom"
	"github.com/caasmo/restinpieces/server"
	"github.com/caasmo/restinpieces/setup"
)

// --- Global Flags ---
var (
	globalVerbose *bool
)

// --- Command Handlers ---

func handleBootstrap(args []string) error {
	// 1. Create FlagSet
	bootstrapCmd := flag.NewFlagSet("bootstrap", flag.ExitOnError)

	// 2. Define flags
	configFile := bootstrapCmd.String("config", "", "Configuration file path")
	dbFile := bootstrapCmd.String("db", "", "Database file path")
	env := bootstrapCmd.String("env", "development", "Environment (development|staging|production)")
	files := bootstrapCmd.String("files", "", "Files directory path")
	dbfile := bootstrapCmd.String("dbfile", "bench.db", "SQLite database file path")

	// 3. Custom usage
	bootstrapCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s bootstrap [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Initialize application resources\n\n")
		bootstrapCmd.PrintDefaults()
	}

	// 4. Parse args
	if err := bootstrapCmd.Parse(args); err != nil {
		return err
	}

	// 5. Placeholder logic
	fmt.Printf("Bootstrap command called with config=%s, db=%s, env=%s, files=%s, dbfile=%s\n",
		*configFile, *dbFile, *env, *files, *dbfile)
	return nil
}

func handleServe(args []string) error {
	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	dbfile := serveCmd.String("dbfile", "bench.db", "SQLite database file path")
	
	serveCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s serve [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Start the application server\n\n")
		serveCmd.PrintDefaults()
	}

	if err := serveCmd.Parse(args); err != nil {
		return err
	}

	// Load initial configuration
	cfg, err := config.Load(*dbfile)
	if err != nil {
		slog.Error("failed to load initial config", "error", err)
		return err
	}

	// Create the config provider with the initial config
	configProvider := config.NewProvider(cfg)

	app, proxy, err := setup.SetupApp(configProvider)
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		return err
	}
	defer app.Close()

	// Log embedded assets using the app's logger
	app.Logger().Debug("logging embedded assets", "public_dir", cfg.PublicDir)
	logEmbeddedAssets(restinpieces.EmbeddedAssets, cfg, app.Logger())

	// Setup custom app
	cApp := custom.NewApp(app)

	// Setup routing
	route(cfg, app, cApp)

	// Setup Scheduler
	scheduler, err := setup.SetupScheduler(configProvider, app.Db(), app.Logger())
	if err != nil {
		return err
	}

	// Start the server
	srv := server.NewServer(configProvider, proxy, scheduler, app.Logger())
	srv.Run()
	return nil
}

func handleDumpConfig(args []string) error {
	dumpCmd := flag.NewFlagSet("dump-config", flag.ExitOnError)
	output := dumpCmd.String("output", "", "Output file path")

	dumpCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s dump-config [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Dump current configuration\n\n")
		dumpCmd.PrintDefaults()
	}

	if err := dumpCmd.Parse(args); err != nil {
		return err
	}

	// Placeholder logic
	fmt.Printf("Dump-config command called with output=%s\n", *output)
	return nil
}

func handleLoadConfig(args []string) error {
	loadCmd := flag.NewFlagSet("load-config", flag.ExitOnError)
	input := loadCmd.String("input", "", "Input file path")

	loadCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s load-config [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Load configuration from file\n\n")
		loadCmd.PrintDefaults()
	}

	if err := loadCmd.Parse(args); err != nil {
		return err
	}

	// Placeholder logic
	fmt.Printf("Load-config command called with input=%s\n", *input)
	return nil
}

func logEmbeddedAssets(assets fs.FS, cfg *config.Config, logger *slog.Logger) {
	subFS, err := fs.Sub(assets, cfg.PublicDir)
	if err != nil {
		logger.Error("failed to create sub filesystem for logging assets", "error", err)
		return // Or handle the error more gracefully
	}
	assetCount := 0
	fs.WalkDir(subFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			assetCount++
			logger.Debug("embedded asset", "path", path)
		}
		return nil
	})
	logger.Debug("total embedded assets", "count", assetCount)
}

func main() {
	// --- 1. Define GLOBAL flags ---
	globalVerbose = flag.Bool("v", false, "Enable global verbose output")

	// Set custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-v] <command> [command-flags] [arguments...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Available commands:\n")
		fmt.Fprintf(os.Stderr, "  bootstrap     Initialize application resources\n")
		fmt.Fprintf(os.Stderr, "  serve         Start the application server\n")
		fmt.Fprintf(os.Stderr, "  dump-config   Dump current configuration\n")
		fmt.Fprintf(os.Stderr, "  load-config   Load configuration from file\n")
		fmt.Fprintf(os.Stderr, "\nUse \"%s <command> -h\" for command-specific help.\n", os.Args[0])
	}

	// --- 2. Parse GLOBAL flags ---
	flag.Parse()

	// --- 3. Get remaining args ---
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: No command specified.")
		flag.Usage()
		os.Exit(1)
	}

	// --- 4. Dispatch command ---
	command := args[0]
	commandArgs := args[1:]

	var err error
	switch command {
	case "bootstrap":
		err = handleBootstrap(commandArgs)
	case "serve":
		err = handleServe(commandArgs)
	case "dump-config":
		err = handleDumpConfig(commandArgs)
	case "load-config":
		err = handleLoadConfig(commandArgs)
	default:
		err = fmt.Errorf("unknown command '%s'", command)
		flag.Usage()
	}

	// --- 5. Handle errors ---
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Error running command '%s': %v\n", command, err)
		os.Exit(1)
	}
}
