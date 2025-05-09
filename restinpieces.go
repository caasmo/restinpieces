package restinpieces

import (
	"fmt"
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
	"github.com/caasmo/restinpieces/notify/discord"
	"github.com/caasmo/restinpieces/queue/handlers"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
	"github.com/caasmo/restinpieces/router"
	"github.com/caasmo/restinpieces/server"
	"github.com/pelletier/go-toml/v2"
)

// New creates a new App instance and Server with the provided options and age key file path.
// It initializes the core application components like database, router, cache first,
// then loads configuration from the database using the provided age key.
func New(opts ...core.Option) (*core.App, *server.Server, error) {
	app, err := core.NewApp(opts...)
	if err != nil {
		slog.Error("failed to initialize core app", "error", err)
		return nil, nil, err
	}

	// Load config from database
	scope := config.ScopeApplication
	decryptedBytes, err := app.SecureConfigStore().Latest(scope)
	if err != nil {
		app.Logger().Error("failed to load/decrypt config", "error", err)
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Unmarshal TOML
	cfg := &config.Config{}
	if err := toml.Unmarshal(decryptedBytes, cfg); err != nil {
		app.Logger().Error("failed to unmarshal config", "error", err)
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate config
	if err := config.Validate(cfg); err != nil {
		app.Logger().Error("config validation failed", "error", err)
		return nil, nil, fmt.Errorf("invalid config: %w", err)
	}

	cfg.Source = "" // Clear source field

	configProvider := config.NewProvider(cfg)
	app.SetConfigProvider(configProvider)
	//app.Logger().Info("config", "config", cfg)

	// Setup custom application logic and routes
	route(cfg, app)

	scheduler, err := SetupScheduler(configProvider, app.DbAuth(), app.DbQueue(), app.Logger())
	if err != nil {
		app.Logger().Error("failed to setup scheduler", "error", err)
		return nil, nil, err
	}

	// Initialize notifier if configured
	if cfg.Notifier.Discord.Activated {
		discordNotifier, err := discord.New(discord.Options{
			WebhookURL:   cfg.Notifier.Discord.WebhookURL,
			APIRateLimit: cfg.Notifier.Discord.APIRateLimit.Duration,
			APIBurst:     cfg.Notifier.Discord.APIBurst,
			SendTimeout:  cfg.Notifier.Discord.SendTimeout.Duration,
		}, app.Logger())
		if err != nil {
			app.Logger().Error("failed to initialize Discord notifier", "error", err)
			return nil, nil, fmt.Errorf("failed to initialize Discord notifier: %w", err)
		}
		app.WithNotifier(discordNotifier)
	} else {
		app.WithNotifier(notify.NewNilNotifier())
	}

	// Initialize the PreRouter chain with internal middleware
	preRouterHandler := initPreRouter(app)

	srv := server.NewServer(
		configProvider,
		preRouterHandler,
		app.Logger(),
	)

	// Register the framework's core daemons
	srv.AddDaemon(scheduler)

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
	// Execution order will be: BlockIp -> BlockUa -> TLSHeaderSTS -> Maintenance -> app.Router()

	// 1. BlockIp Middleware (Added first, runs first)
	if cfg.BlockIp.Enabled {
		// Instantiate using app resources
		blockIp := prerouter.NewBlockIp(app.Cache(), logger) // Keep logger for BlockIp
		preRouterChain.WithMiddleware(blockIp.Execute)
		logger.Info("Prerouter Middleware BlockIp enabled")
	} else {
		logger.Info("Prerouter Middleware BlockIp disabled")
	}

	// 2. BlockUa Middleware (Added second, runs second)
	if cfg.BlockUa.Activated {
		// Instantiate using app instance
		blockUa := prerouter.NewBlockUa(app)
		preRouterChain.WithMiddleware(blockUa.Execute)
		logger.Info("Prerouter Middleware BlockUa enabled")
	} else {
		logger.Info("Prerouter Middleware BlockUa disabled")
	}

	// 3. TLSHeaderSTS Middleware (Added third, runs third)
	// This should run early to ensure HSTS is set for TLS requests, but after IP/UA blocking.
	tlsHeaderSTS := prerouter.NewTLSHeaderSTS()
	preRouterChain.WithMiddleware(tlsHeaderSTS.Execute)
	// No specific log for TLSHeaderSTS as it always runs

	// 4. Maintenance Middleware (Added fourth, runs fourth)
	// Always added; behavior controlled by cfg.Maintenance.Activated
	maintenance := prerouter.NewMaintenance(app)
	preRouterChain.WithMiddleware(maintenance.Execute)
	logger.Info("Prerouter Middleware Maintenance added (activation depends on config)")

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
