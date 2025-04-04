package setup

import (
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/core/proxy"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/executor"
	"github.com/caasmo/restinpieces/queue/handlers"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/db"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
)


func SetupApp(cfg *config.Config) (*core.App, *proxy.Proxy, error) {

	// Create the App instance first (without the proxy)
	app, err := core.NewApp(
		WithDBCrawshaw(cfg.DBFile),
		WithRouterServeMux(),
		WithCacheRistretto(),
		core.WithConfig(cfg),
		//WithPhusLogger(nil), // Provide the logger using defaults
		WithTextLogger(nil), // Provide the logger using defaults
	)
	if err != nil {
		return nil, nil, err
	}

	// Create the Proxy instance, passing the app and config
	px := proxy.NewProxy(app, cfg)

	// Return the app, the proxy, and no error
	return app, px, nil
}

// to file setup_scheduler
func SetupScheduler(cfg *config.Config, db db.Db, logger *slog.Logger) (*scl.Scheduler, error) {

	hdls := make(map[string]executor.JobHandler)

	if (cfg.Smtp != config.Smtp{}) {
		mailer, err := mail.New(cfg.Smtp)
		if err != nil {
			logger.Error("failed to create mailer", "error", err)
			os.Exit(1)
		}

		emailVerificationHandler := handlers.NewEmailVerificationHandler(db, cfg, mailer)
		hdls[queue.JobTypeEmailVerification] = emailVerificationHandler

		passwordResetHandler := handlers.NewPasswordResetHandler(db, cfg, mailer)
		hdls[queue.JobTypePasswordReset] = passwordResetHandler

		emailChangeHandler := handlers.NewEmailChangeHandler(db, cfg, mailer)
		hdls[queue.JobTypeEmailChange] = emailChangeHandler
	}

	return scl.NewScheduler(cfg.Scheduler, db, executor.NewExecutor(hdls), logger), nil

}
