package restinpieces

import (
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/core/proxy"
	"github.com/caasmo/restinpieces/custom"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/executor"
	"github.com/caasmo/restinpieces/queue/handlers"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
	"github.com/caasmo/restinpieces/server"
)

// New creates a new App instance and Server with the provided options.
// It initializes the core application components like database, router, cache first,
// then loads configuration either from TOML file (if path provided) or DB file.
func New(dbfile string, configPath string, opts ...core.Option) (*core.App, *server.Server, error) {
	// First create app without config
	app, err := core.NewApp(opts...)
	if err != nil {
		slog.Error("failed to initialize core app", "error", err)
		return nil, nil, err
	}

	// Now load config using the initialized DB drivers
	var cfg *config.Config
	if configPath != "" {
		cfg, err = config.LoadFromToml(configPath, dbfile)
	} else {
		cfg, err = config.LoadFromDb(dbfile)
	}

	if err != nil {
		slog.Error("failed to load config", "error", err)
		return nil, nil, err
	}

	configProvider := config.NewProvider(cfg)
	app.SetConfigProvider(configProvider)

	// Create the Proxy instance, passing the app
	px := proxy.NewProxy(app)

	// Setup custom application logic and routes
	cApp := custom.NewApp(app)
	route(cfg, app, cApp) // Assuming route function exists and is correctly defined elsewhere

	scheduler, err := SetupScheduler(configProvider, app.DbAuth(), app.DbQueue(), app.Logger())
	if err != nil {
		// app.Close() // Removed as DB lifecycle is managed externally
		slog.Error("failed to setup scheduler", "error", err)
		return nil, nil, err
	}

	// Create the server instance
	srv := server.NewServer(configProvider, px, scheduler, app.Logger())

	// Return the initialized app and server
	return app, srv, nil
}

func SetupScheduler(configProvider *config.Provider, dbAuth db.DbAuth, dbQueue db.DbQueue, logger *slog.Logger) (*scl.Scheduler, error) {

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

		emailVerificationHandler := handlers.NewEmailVerificationHandler(dbAuth, configProvider, mailer)
		hdls[queue.JobTypeEmailVerification] = emailVerificationHandler

		passwordResetHandler := handlers.NewPasswordResetHandler(dbAuth, configProvider, mailer)
		hdls[queue.JobTypePasswordReset] = passwordResetHandler

		emailChangeHandler := handlers.NewEmailChangeHandler(dbAuth, configProvider, mailer)
		hdls[queue.JobTypeEmailChange] = emailChangeHandler
	}

	return scl.NewScheduler(configProvider, dbQueue, executor.NewExecutor(hdls), logger), nil
}
