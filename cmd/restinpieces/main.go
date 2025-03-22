package main

import (
	"flag"
	"log/slog"
	"os"

	// TODO move to init
	"github.com/caasmo/restinpieces/custom"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/executor"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/server"
)

func main() {

	dbfile := flag.String("dbfile", "bench.db", "SQLite database file path")
	flag.Parse()

	cfg, err := config.Load(*dbfile)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ap, err := initApp(cfg)
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	// TODO better custom/app move to init_app
	cAp := custom.NewApp(ap)

	// TODO with custom
	defer ap.Close()

	route(ap, cAp)

	// Create mailer and executor
	mailer := mail.New(cfg.Smtp)
	handlers := map[string]executor.JobHandler{
		queue.JobTypeEmailVerification: mailer,
	}
	exec := executor.NewExecutor(handlers)

	// Create and start scheduler with executor
	scheduler := scl.NewScheduler(cfg.Scheduler, ap.Db(), exec)

	//server.Run(cfg.Server, ap.Router(), nil)
	server.Run(cfg.Server, ap.Router(), scheduler)
}
