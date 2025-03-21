package main

import (
	"flag"
	"log/slog"
	"os"
	"time"

	// TODO move to init
	"github.com/caasmo/restinpieces/custom"

	"github.com/caasmo/restinpieces/config"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
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

	// Create and start scheduler with configured interval and db 
	scheduler := scl.NewScheduler(cfg.Scheduler, ap.Db())
	
	server.Run(cfg.Server, ap.Router(), scheduler)
}
