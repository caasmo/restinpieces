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

// SetupApp initializes the core application components using the provided config provider.
func SetupApp(configProvider *config.Provider, dbFile string) (*core.App, *proxy.Proxy, error) {

	// Create the App instance first (without the proxy)
	// Logger is now configured via core.WithLogger() option below
	app, err := core.NewApp(
		// Database setup needs the file path from the *initial* config,
		// as the DB connection is typically established once at startup.
		// If DB config needs to be dynamic, that's a more complex scenario.
		WithDBCrawshaw(dbFile), // Use dbFile passed from main
		WithRouterServeMux(),
		WithCacheRistretto(),
		core.WithConfigProvider(configProvider), // Pass the provider
		core.WithLogger(),                       // Initialize logger using default setup option
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

// SetupScheduler initializes the job scheduler.
// It currently accepts the initial *config.Config.
// TODO: Update this to accept *config.Provider later.
func SetupScheduler(cfg *config.Config, db db.Db, logger *slog.Logger) (*scl.Scheduler, error) {

	hdls := make(map[string]executor.JobHandler)

	// Get the config snapshot for setup (using the passed initial cfg)
	currentCfg := cfg // Use the initial config passed as argument

	// Setup mailer only if SMTP is configured in the current config
	if (currentCfg.Smtp != config.Smtp{}) {

		// Note: Mailer itself might not be easily hot-reloadable if connection details change.
		// If SMTP settings need to be dynamic, the mailer creation/logic might need adjustment,
		// potentially recreating the mailer inside the job handlers when needed.
		mailer, err := mail.New(currentCfg.Smtp)
		if err != nil {
			logger.Error("failed to create mailer", "error", err)
			// Decide if this is fatal. If mailing is optional, maybe just log and continue without mail handlers?
			// For now, let's treat it as fatal if configured but failing.
			os.Exit(1) // Or return err
		}

		// Pass the initial config (cfg) to handlers for now
		// TODO: Update handlers to accept *config.Provider later
		emailVerificationHandler := handlers.NewEmailVerificationHandler(db, cfg, mailer)
		hdls[queue.JobTypeEmailVerification] = emailVerificationHandler

		passwordResetHandler := handlers.NewPasswordResetHandler(db, cfg, mailer)
		hdls[queue.JobTypePasswordReset] = passwordResetHandler

		emailChangeHandler := handlers.NewEmailChangeHandler(db, cfg, mailer)
		hdls[queue.JobTypeEmailChange] = emailChangeHandler
	}

	// Pass the initial config (cfg.Scheduler) to the scheduler itself for now
	// TODO: Update scheduler to accept *config.Provider later
	return scl.NewScheduler(cfg.Scheduler, db, executor.NewExecutor(hdls), logger), nil
}
