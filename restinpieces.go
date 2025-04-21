package restinpieces

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/core/prerouter"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/executor"
	"github.com/caasmo/restinpieces/queue/handlers" // Keep for other handlers
	scl "github.com/caasmo/restinpieces/queue/scheduler"
	"github.com/caasmo/restinpieces/router"
	"github.com/caasmo/restinpieces/server"
)

// New creates a new App instance and Server with the provided options and age key file path.
// It initializes the core application components like database, router, cache first,
// then loads configuration from the database using the provided age key.
func New(ageKeyPath string, opts ...core.Option) (*core.App, *server.Server, error) {
	// First create app without config
	app, err := core.NewApp(opts...)
	if err != nil {
		slog.Error("failed to initialize core app", "error", err)
		return nil, nil, err
	}

	appLogger := app.Logger() // Get logger once
	// Pass ageKeyPath to LoadFromDb
	cfg, err := config.LoadFromDb(app.DbConfig(), appLogger, ageKeyPath)
	if err == nil { // Only set source on success
		cfg.Source = "" // empty for db
	}

	if err != nil {
		app.Logger().Error("failed to load config", "error", err)
		return nil, nil, err
	}

	configProvider := config.NewProvider(cfg)
	app.SetConfigProvider(configProvider)
	app.Logger().Info("config", "config", cfg)

	// Setup custom application logic and routes
	route(cfg, app) // Assuming route function exists and is correctly defined elsewhere

	// SetupScheduler no longer needs DbAcme
	scheduler, err := SetupScheduler(configProvider, app.DbAuth(), app.DbQueue(), app.Logger())
	if err != nil {
		app.Logger().Error("failed to setup scheduler", "error", err)
		return nil, nil, err
	}

	// Create the server instance, passing 'app' as the http.Handler
	// Initialize the PreRouter chain with internal middleware
	preRouterHandler := initPreRouter(app)

	// Create the server instance (without daemons initially)
	srv := server.NewServer(
		configProvider,
		preRouterHandler, // The main HTTP handler chain
		app.Logger(),
	)

	// Register the framework's core daemons
	srv.AddDaemon(scheduler)

	// Return the initialized app and server
	return app, srv, nil
}

// initPreRouter sets up the internal pre-router middleware chain based on configuration
// and returns the final http.Handler.
// No User Pre-Router Customization:
// we allow disable in config, we do not allow adding, the user can put in normal middleware.
// configure the framework's pre-router features; add your own logic at the route level.
// Framework handles everything before routing; user handles everything after routing
func initPreRouter(app *core.App) http.Handler {
	logger := app.Logger()
	cfg := app.Config()

	// Start the chain with the application's main router as the base handler.
	// The final handler in the chain will be app.Router().ServeHTTP
	preRouterChain := router.NewChain(app.Router())

	// --- Add Internal Middleware Conditionally (Order Matters!) ---
	// Execution order will be: BlockIp -> TLSHeaderSTS -> Maintenance -> app.Router()

	// 1. BlockIp Middleware (Added first, runs first)
	if cfg.BlockIp.Enabled {
		// Instantiate using app resources
		blockIp := prerouter.NewBlockIp(app.Cache(), logger) // Keep logger for BlockIp
		preRouterChain.WithMiddleware(blockIp.Execute)
		logger.Info("Prerouter Middleware BlockIp enabled")
	} else {
		logger.Info("Prerouter Middleware BlockIp disabled")
	}

	// 2. TLSHeaderSTS Middleware (Added second, runs second)
	// This should run early to ensure HSTS is set for TLS requests, but after IP blocking.
	tlsHeaderSTS := prerouter.NewTLSHeaderSTS()
	preRouterChain.WithMiddleware(tlsHeaderSTS.Execute)
	// No specific log for TLSHeaderSTS as it always runs

	// 3. Maintenance Middleware (Added third, runs third)
	if cfg.Maintenance.Enabled {
		// Instantiate using app instance (no logger needed)
		maintenance := prerouter.NewMaintenance(app)
		preRouterChain.WithMiddleware(maintenance.Execute)
		logger.Info("Prerouter Middleware Maintenance enabled")
	} else {
		logger.Info("Prerouter Middleware Maintenance disabled")
	}

	// --- Finalize the PreRouter ---
	preRouterHandler := preRouterChain.Handler()
	logger.Info("PreRouter handler chain configured")

	return preRouterHandler
}

// SetupScheduler initializes the job scheduler and its handlers.
// dbAcme parameter removed.
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

	// ACME handler registration removed.

	return scl.NewScheduler(configProvider, dbQueue, executor.NewExecutor(hdls), logger), nil
}
