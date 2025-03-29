package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/custom"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/executor"
	"github.com/caasmo/restinpieces/queue/handlers"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
	"github.com/caasmo/restinpieces/server"
)

func main() {
	// Initialize logging
	logHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(logHandler))

	dbfile := flag.String("dbfile", "bench.db", "SQLite database file path")
	flag.Parse()

	cfg, err := config.Load(*dbfile)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Debug("embedded assets", "count", len(restinpieces.EmbeddedAssets))

	ap, err := initApp(cfg)
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	// TODO better custom/app move to init_app
	cAp := custom.NewApp(ap)

	// TODO with custom
	defer ap.Close()

	route(cfg, ap, cAp)

	// Create mailer and executor only if SMTP is configured
	hdls := make(map[string]executor.JobHandler)

	if (cfg.Smtp != config.Smtp{}) {
		mailer, err := mail.New(cfg.Smtp)
		if err != nil {
			slog.Error("failed to create mailer", "error", err)
			os.Exit(1)
		}

		emailVerificationHandler := handlers.NewEmailVerificationHandler(ap.Db(), cfg, mailer)
		hdls[queue.JobTypeEmailVerification] = emailVerificationHandler
	}

	scheduler := scl.NewScheduler(cfg.Scheduler, ap.Db(), executor.NewExecutor(hdls))

	//server.Run(cfg.Server, ap.Router(), nil)
	server.Run(cfg.Server, ap.Router(), scheduler)
}
