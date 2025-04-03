package main

import (
	"flag"
	"io/fs"
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/custom"
	"github.com/caasmo/restinpieces/core/proxy"
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

	cfg, err := config.Load(*dbfile)
	if err != nil {
		//app.Logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// DOcumetn App is services and standard enpoints
	app, err := setup.SetupApp(cfg)
	defer app.Close()
	if err != nil {
		//app.Logger.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	// Log embedded assets
	app.Logger().Debug("logging embedded assets", "public_dir", cfg.PublicDir)
	logEmbeddedAssets(restinpieces.EmbeddedAssets, cfg, app.Logger())

	// TODO better custom/app move to init_app
	cApp := custom.NewApp(app)

	// TODO with custom
	route(cfg, app, cApp)


	// TODO
	scheduler, err := setup.SetupScheduler(cfg, app.Db(), app.Logger())
	if err != nil {
		//app.Logger.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	// proxi is of the app TODO app.Proxy
	proxy := proxy.NewProxy(app)
	server.Run(cfg.Server, proxy, scheduler, app.Logger())
}
