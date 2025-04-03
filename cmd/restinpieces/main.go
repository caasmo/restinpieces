package main

import (
	"flag"
	"io/fs"
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/custom"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/proxy"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/executor"
	"github.com/caasmo/restinpieces/queue/handlers"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
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

	app, err := initApp(cfg)
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

	// Create mailer and executor only if SMTP is configured
	hdls := make(map[string]executor.JobHandler)

	if (cfg.Smtp != config.Smtp{}) {
		mailer, err := mail.New(cfg.Smtp)
		if err != nil {
			app.Logger().Error("failed to create mailer", "error", err)
			os.Exit(1)
		}

		emailVerificationHandler := handlers.NewEmailVerificationHandler(app.Db(), cfg, mailer)
		hdls[queue.JobTypeEmailVerification] = emailVerificationHandler

		passwordResetHandler := handlers.NewPasswordResetHandler(app.Db(), cfg, mailer)
		hdls[queue.JobTypePasswordReset] = passwordResetHandler

		emailChangeHandler := handlers.NewEmailChangeHandler(app.Db(), cfg, mailer)
		hdls[queue.JobTypeEmailChange] = emailChangeHandler
	}

	scheduler := scl.NewScheduler(cfg.Scheduler, app.Db(), executor.NewExecutor(hdls), app.Logger())

	proxy := proxy.NewProxy(app.Router(), cfg)
	server.Run(cfg.Server, proxy, scheduler, app.Logger())
}
