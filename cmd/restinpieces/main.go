package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	// "context" // Not currently used, uncomment if createZombiezenPool is used
	"os"
	"runtime" // Required for pool size calculation
	// "time" // Not currently used

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	// "github.com/caasmo/restinpieces/core" // Not currently used
	//"github.com/caasmo/restinpieces/custom"
	//"github.com/caasmo/restinpieces/server"

	crawshawPool "crawshaw.io/sqlite/sqlitex"
	zombiezenPool "zombiezen.com/go/sqlite/sqlitex"
)


// --- Pool Creation Helpers ---

func createCrawshawPool(dbPath string) (*crawshawPool.Pool, error) {

    // TODO documetn option requiring wal for example for litestream
    p, err := sqlitex.Open(initString, 0, poolSize)
	poolSize := runtime.NumCPU()
    initString := fmt.Sprintf("file:%s", path)

	pool, err := crawshawPool.Open(initString, 0, poolSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create crawshaw pool at %s: %w", dbPath, err)
	}

	// Optional: Ping the pool to ensure connectivity
	// conn := pool.Get(nil)
	// if conn == nil {
	//  pool.Close()
	//  return nil, fmt.Errorf("failed to get connection from new crawshaw pool")
	// }
	// pool.Put(conn)
	slog.Info("Crawshaw pool created successfully", "path", dbPath)
	return pool, nil
}

func createZombiezenPool(dbPath string) (*zombiezenPool.Pool, error) {
	poolSize := runtime.NumCPU()
	// Match the settings used in zombiezen.New for consistency
	initString := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)

	pool, err := zombiezenPool.NewPool(initString, zombiezenPool.PoolOptions{
		PoolSize: poolSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create zombiezen pool at %s: %w", dbPath, err)
	}
	// Optional: Ping the pool
	// conn, err := pool.Take(context.Background())
	// if err != nil {
	//  pool.Close()
	//  return nil, fmt.Errorf("failed to get connection from new zombiezen pool: %w", err)
	// }
	// pool.Put(conn)
	slog.Info("Zombiezen pool created successfully", "path", dbPath)
	return pool, nil
}


// --- Command Handlers ---

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

	// --- Create the Database Pool ---
	// Choose which pool to create (e.g., Crawshaw)
	dbPool, err := createCrawshawPool(*dbfile)
	// Or: dbPool, err := createZombiezenPool(*dbfile)
	if err != nil {
		slog.Error("failed to create database pool", "error", err)
		return err
	}
	// Defer closing the pool here, as the user (main) owns it now.
	// This must happen *after* app.Close() finishes.
	defer func() {
		slog.Info("Closing database pool...")
		if err := dbPool.Close(); err != nil {
			slog.Error("Error closing database pool", "error", err)
		}
	}()


	// --- Initialize the Application ---
	// Pass the *existing* pool using the new option
	app, srv, err := restinpieces.New(
		*dbfile, // dbfile might still be needed for config loading? Review restinpieces.New
		// Use the appropriate option for the pool type created above
		restinpieces.WithCrawshawDB(dbPool),
		// Or: restinpieces.WithZombiezenDB(dbPool),
		restinpieces.WithRouterServeMux(),
		restinpieces.WithCacheRistretto(),
		restinpieces.WithTextLogger(nil),
	)
	if err != nil {
		slog.Error("failed to initialize application", "error", err)
		// Pool will be closed by the deferred function
		return err
	}
	// app.Close() will now call the Db implementation's Close,
	// which *won't* close the pool itself. The pool closure is handled by the defer above.
	defer app.Close()


	// Log embedded assets (if needed)
	//logEmbeddedAssets(restinpieces.EmbeddedAssets, app.Config(), app.Logger())

	// Start the server
	// Consider passing context for graceful shutdown coordination with pool closing
	srv.Run()

	slog.Info("Server shut down gracefully.")
	return nil // Error is handled before returning nil
}

func handleBootstrap(args []string) error {
	// 1. Create FlagSet
	bootstrapCmd := flag.NewFlagSet("bootstrap", flag.ExitOnError)

	// 2. Define flags
	config := bootstrapCmd.Bool("config", false, "Initialize configuration")
	db := bootstrapCmd.Bool("db", false, "Initialize database")
	env := bootstrapCmd.Bool("env", false, "Initialize environment")
	files := bootstrapCmd.Bool("files", false, "Initialize files directory")
	dbfile := bootstrapCmd.String("dbfile", "bench.db", "SQLite database file path")

	// 3. Custom usage
	bootstrapCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s bootstrap [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Initialize application resources\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fmt.Fprintf(os.Stderr, "  --config        Initialize configuration\n")
		fmt.Fprintf(os.Stderr, "  --db           Initialize database\n")
		fmt.Fprintf(os.Stderr, "  --env          Initialize environment\n")
		fmt.Fprintf(os.Stderr, "  --files        Initialize files directory\n")
		fmt.Fprintf(os.Stderr, "  --dbfile string   SQLite database file (default \"bench.db\")\n\n")
		fmt.Fprintf(os.Stderr, "All flags except --dbfile are boolean switches\n")
	}

	// 4. Parse args
	if err := bootstrapCmd.Parse(args); err != nil {
		return err
	}

	// 5. Placeholder logic
	fmt.Printf("Bootstrap command called with config=%t, db=%t, env=%t, files=%t, dbfile=%s\n",
		*config, *db, *env, *files, *dbfile)
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
	// No global flags defined here

	// Set custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-v] [command] [command-flags] [arguments...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Available commands:\n")
		fmt.Fprintf(os.Stderr, "  (default)      Start the application server (same as 'serve')\n")
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
	var command string
	var commandArgs []string

	if len(args) < 1 {
		// Default to serve command if none specified
		command = "serve"
		commandArgs = []string{}
	} else {
		command = args[0]
		commandArgs = args[1:]
	}

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
