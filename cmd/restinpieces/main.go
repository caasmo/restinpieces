package main

import (
	"flag"
	"io/fs"
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/custom"
	"github.com/caasmo/restinpieces/setup"
	"github.com/caasmo/restinpieces/server"
)

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

	dbfile := flag.String("dbfile", "bench.db", "SQLite database file path")
	flag.Parse()

	// Load initial configuration
	cfg, err := config.Load(*dbfile)
	if err != nil {
		slog.Error("failed to load initial config", "error", err) // Use default logger before app logger is ready
		os.Exit(1)
	}

	// Create the config provider with the initial config
	configProvider := config.NewProvider(cfg)

	// Setup App, passing the provider and the db file path
	// Note: SetupApp needs the logger, let's create one here
	// TODO: Make logger configurable
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	app, proxy, err := setup.SetupApp(configProvider, *dbfile) // Pass provider and dbfile
	if err != nil {
		logger.Error("failed to initialize app", "error", err) // Use the created logger
		os.Exit(1)
		os.Exit(1)
	}
	defer app.Close() // Defer close after successful app initialization

	// Log embedded assets using the app's logger
	app.Logger().Debug("logging embedded assets", "public_dir", cfg.PublicDir) // Use initial cfg here is fine
	logEmbeddedAssets(restinpieces.EmbeddedAssets, cfg, app.Logger())

	// TODO better custom/app move to init_app
	cApp := custom.NewApp(app)

	// Setup routing - Pass initial config for setup
	route(cfg, app, cApp)

	// Setup Scheduler - Pass initial config (cfg) for now
	scheduler, err := setup.SetupScheduler(cfg, app.Db(), app.Logger())
	if err != nil {
		app.Logger().Error("failed to initialize scheduler", "error", err)
		os.Exit(1)
	}

	// Create Server - Pass initial server config (cfg.Server) for now
	srv := server.NewServer(cfg.Server, proxy, scheduler, app.Logger())

	// Run the server (blocking call)
	app.Logger().Info("starting server", "addr", cfg.Server.Addr)
	if err := srv.Run(); err != nil {
		app.Logger().Error("server run failed", "error", err)
		os.Exit(1) // Exit if server fails to run
	}
}
