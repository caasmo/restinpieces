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
	"github.com/caasmo/restinpieces/server"
)

// New creates a new App instance and Server with the provided options.
// It initializes the core application components like database, router, cache first,
// then loads configuration either from TOML file (if path provided) or DB file.
func New(configPath string, opts ...core.Option) (*core.App, *server.Server, error) {
	// First create app without config
	app, err := core.NewApp(opts...)
	if err != nil {
		slog.Error("failed to initialize core app", "error", err)
		return nil, nil, err
	}

	var cfg *config.Config
	appLogger := app.Logger() // Get logger once
	if configPath != "" {
		cfg, err = config.LoadFromToml(configPath, appLogger)
		if err == nil { // Only set source on success
			cfg.Source = configPath
		}
	} else {
		cfg, err = config.LoadFromDb(app.DbConfig(), app.DbAcme(), appLogger)
		if err == nil { // Only set source on success
			cfg.Source = "" // empty for db
		}
	}

	if err != nil {
		app.Logger().Error("failed to load config", "error", err)
		return nil, nil, err
	}

	configProvider := config.NewProvider(cfg)
	app.SetConfigProvider(configProvider)

	// Create the Proxy instance, passing the app
	px := proxy.NewProxy(app)

	// Setup custom application logic and routes
	route(cfg, app) // Assuming route function exists and is correctly defined elsewhere

	// Pass DbAcme to SetupScheduler
	scheduler, err := SetupScheduler(configProvider, app.DbAuth(), app.DbQueue(), app.DbAcme(), app.Logger())
	if err != nil {
		app.Logger().Error("failed to setup scheduler", "error", err)
		return nil, nil, err
	}

	// Create the server instance
	srv := server.NewServer(configProvider, px, scheduler, app.Logger())

	// Return the initialized app and server
	return app, srv, nil
}

// SetupScheduler initializes the job scheduler and its handlers.
// It now requires db.DbAcme to pass to the ACME renewal handler.
func SetupScheduler(configProvider *config.Provider, dbAuth db.DbAuth, dbQueue db.DbQueue, dbAcme db.DbAcme, logger *slog.Logger) (*scl.Scheduler, error) {

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

	// Instantiate and register the TLS Cert Renewal Handler if ACME is enabled in config
	// Note: We check cfg.Acme.Enabled here to avoid unnecessary instantiation if not used.
	// The handler itself also checks this, but this prevents adding it to the map if globally disabled.
	if cfg.Acme.Enabled {
		// Pass dbAcme to the handler constructor
		tlsCertRenewalHandler := handlers.NewTLSCertRenewalHandler(configProvider, dbAcme, logger)
		hdls[queue.JobTypeTLSCertRenewal] = tlsCertRenewalHandler
		logger.Info("Registered TLSCertRenewalHandler")
	} else {
		logger.Info("ACME is disabled, skipping registration of TLSCertRenewalHandler")
	}

	return scl.NewScheduler(configProvider, dbQueue, executor.NewExecutor(hdls), logger), nil
}
