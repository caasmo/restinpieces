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

	cfg := configProvider.Get()

	app, err := core.NewApp(
		WithDBCrawshaw(cfg.DBFile),
		WithRouterServeMux(),
		WithCacheRistretto(),
		core.WithConfigProvider(configProvider),
		//WithPhusLogger(nil), // Provide the logger using defaults
		WithTextLogger(nil), // Provide the logger using defaults
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
// It now accepts a ConfigProvider to access configuration dynamically.
func SetupScheduler(configProvider *config.Provider, db db.Db, logger *slog.Logger) (*scl.Scheduler, error) {

	hdls := make(map[string]executor.JobHandler)

	// Get the current config snapshot for setup (e.g., for mailer)
	currentCfg := configProvider.Get()

	// Setup mailer only if SMTP is configured in the current config
	if (currentCfg.Smtp != config.Smtp{}) {

		mailer, err := mail.New(currentCfg.Smtp) // Use currentCfg here
		if err != nil {
			logger.Error("failed to create mailer", "error", err)
			// Decide if this is fatal. If mailing is optional, maybe just log and continue without mail handlers?
			// For now, let's treat it as fatal if configured but failing.
			os.Exit(1) // Or return err
		}

		// Pass the configProvider to handlers so they can get fresh config if needed
		// TODO: Update handler constructors and implementations later
		emailVerificationHandler := handlers.NewEmailVerificationHandler(db, currentCfg, mailer) // Still passing initial cfg for now
		hdls[queue.JobTypeEmailVerification] = emailVerificationHandler

		passwordResetHandler := handlers.NewPasswordResetHandler(db, currentCfg, mailer) // Still passing initial cfg for now
		hdls[queue.JobTypePasswordReset] = passwordResetHandler

		emailChangeHandler := handlers.NewEmailChangeHandler(db, currentCfg, mailer) // Still passing initial cfg for now
		hdls[queue.JobTypeEmailChange] = emailChangeHandler
	}

	// Pass the configProvider to the scheduler itself
	return scl.NewScheduler(configProvider, db, executor.NewExecutor(hdls), logger), nil
}
