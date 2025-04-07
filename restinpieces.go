package restinpieces

import (
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/core/proxy"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/executor"
	"github.com/caasmo/restinpieces/queue/handlers"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
)

// New creates a new App instance with the provided options.
// It initializes the core application components like database, router, cache, etc.
func New(dbfile string, opts ...core.Option) (*core.App, error) {
	// Load initial configuration
	cfg, err := config.Load(dbfile)
	if err != nil {
		slog.Error("failed to load initial config", "error", err)
		// Returning the error directly might not be ideal here.
		// Consider returning (nil, err) to match the function signature.
		// For now, keeping the original logic but this should be reviewed.
		// TODO: Return (nil, err) instead of just err
		return nil, err // Corrected to return (nil, err)
	}

	// Create the config provider with the initial config
	configProvider := config.NewProvider(cfg)

	return nil, nil // Placeholder return
}

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
func SetupScheduler(configProvider *config.Provider, db db.Db, logger *slog.Logger) (*scl.Scheduler, error) {

	hdls := make(map[string]executor.JobHandler)

	cfg := configProvider.Get()

	// Setup mailer only if SMTP is configured in the current config
	if (cfg.Smtp != config.Smtp{}) {

		mailer, err := mail.New(configProvider)
		if err != nil {
			logger.Error("failed to create mailer", "error", err)
			// Decide if this is fatal. If mailing is optional, maybe just log and continue without mail handlers?
			// For now, let's treat it as fatal if configured but failing.
			os.Exit(1) // Or return err
		}

		emailVerificationHandler := handlers.NewEmailVerificationHandler(db, configProvider, mailer) // Pass provider
		hdls[queue.JobTypeEmailVerification] = emailVerificationHandler

		passwordResetHandler := handlers.NewPasswordResetHandler(db, configProvider, mailer) // Pass provider
		hdls[queue.JobTypePasswordReset] = passwordResetHandler

		emailChangeHandler := handlers.NewEmailChangeHandler(db, configProvider, mailer) // Pass provider
		hdls[queue.JobTypeEmailChange] = emailChangeHandler
	}

	return scl.NewScheduler(configProvider, db, executor.NewExecutor(hdls), logger), nil
}

