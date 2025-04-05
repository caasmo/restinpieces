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

func SetupApp(configProvider *config.Provider) (*core.App, *proxy.Proxy, error) {

	initialCfg := configProvider.Get()

	app, err := core.NewApp(
		WithDBCrawshaw(initialCfg.DBFile),
		WithRouterServeMux(),
		WithCacheRistretto(),
		core.WithConfigProvider(configProvider),
		core.WithLogger(nil),                       // Initialize logger using default setup option
	)
	if err != nil {
		return nil, nil, err
	}

	// Create the Proxy instance, passing the app
	// Proxy reads config dynamically via app.Config() -> configProvider.Get()
	px := proxy.NewProxy(app)

	// Return the app, the proxy, and no error
	return app, px, nil
}

func SetupScheduler(cfg *config.Config, db db.Db, logger *slog.Logger) (*scl.Scheduler, error) {

	hdls := make(map[string]executor.JobHandler)

	if (cfg.Smtp != config.Smtp{}) {

		mailer, err := mail.New(currentCfg.Smtp)
		if err != nil {
			logger.Error("failed to create mailer", "error", err)
			// Decide if this is fatal. If mailing is optional, maybe just log and continue without mail handlers?
			// For now, let's treat it as fatal if configured but failing.
			os.Exit(1) // Or return err
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
