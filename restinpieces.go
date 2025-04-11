package restinpieces

import (
	"log/slog"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	// "github.com/caasmo/restinpieces/core/proxy" // Removed proxy import
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/executor"
	"github.com/caasmo/restinpieces/queue/handlers"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
	"github.com/caasmo/restinpieces/server"
	"github.com/caasmo/restinpieces/core/proxy" // Import for BlockIp and MaintenanceMiddleware
	"github.com/caasmo/restinpieces/router"    // Import for NewChain
)

// Import assets package to ensure embedded data is available during init and build.
// The underscore means we only want the side effects (init functions, embedding).
import _ "github.com/caasmo/restinpieces/assets" // Adjust if your module path is different

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

	// Setup custom application logic and routes
	route(cfg, app) // Assuming route function exists and is correctly defined elsewhere

	// Pass DbAcme to SetupScheduler
	scheduler, err := SetupScheduler(configProvider, app.DbAuth(), app.DbQueue(), app.DbAcme(), app.Logger())
	if err != nil {
		app.Logger().Error("failed to setup scheduler", "error", err)
		return nil, nil, err
	}

	// Create the server instance, passing 'app' as the http.Handler
	// Initialize the PreRouter chain with internal middleware
	preRouterHandler := initPreRouter(app)

	// Create the server instance, passing the composed preRouterHandler
	srv := server.NewServer(configProvider, preRouterHandler, scheduler, app.Logger())

	// Return the initialized app and server
	return app, srv, nil
}

// initPreRouter sets up the internal pre-router middleware chain based on configuration
// and returns the final http.Handler.
func initPreRouter(app *core.App) http.Handler {
	logger := app.Logger()
	cfg := app.Config()

	// Start the chain with the application's main router as the base handler.
	// The final handler in the chain will be app.Router().ServeHTTP
	preRouterChain := router.NewChain(app.Router())

	// --- Add Internal Middleware Conditionally (Order Matters!) ---
	// Middlewares are added using WithMiddleware, which prepends them.
	// The last middleware added is the first one to execute.
	// Execution order will be: Maintenance -> BlockIp -> app.Router()

	// 1. BlockIp Middleware (Added first, runs second)
	if cfg.BlockIp.Enabled {
		// Instantiate using app resources
		blockIpInstance := proxy.NewBlockIp(app.Cache(), logger)
		preRouterChain.WithMiddleware(blockIpInstance.Execute)
		logger.Info("Internal Middleware: BlockIp enabled")
	} else {
		logger.Info("Internal Middleware: BlockIp disabled")
	}

	// 2. Maintenance Middleware (Added second, runs first)
	// We check Enabled here for setup, but the middleware itself checks Activated dynamically on each request.
	if cfg.Maintenance.Enabled {
		// Instantiate using app instance (needed for GetClientIP and config)
		maintenanceInstance := proxy.NewMaintenanceMiddleware(app, logger)
		preRouterChain.WithMiddleware(maintenanceInstance.Execute)
		logger.Info("Internal Middleware: Maintenance enabled")
	} else {
		logger.Info("Internal Middleware: Maintenance disabled")
	}


	// 3. Add other internal middleware here (e.g., RateLimiter, Metrics, Logging)
	// These would typically be added *after* Maintenance and BlockIp in this block
	// so they execute *before* them.
	// Example:
	// if cfg.RateLimiter.Enabled {
	//    rateLimiter := internal.NewRateLimiter(...)
	//    preRouterChain.WithMiddleware(rateLimiter.Execute)
	//    logger.Info("Internal Middleware: RateLimiter enabled")
	// }

	// --- Finalize the PreRouter ---
	// Get the final composed handler
	finalPreRouterHandler := preRouterChain.Handler()
	logger.Info("Internal PreRouter handler chain configured")

	// Return the final handler
	return finalPreRouterHandler
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
