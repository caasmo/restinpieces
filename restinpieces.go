package restinpieces

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/core/prerouter"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/log"
	"github.com/caasmo/restinpieces/mail"
	"github.com/caasmo/restinpieces/notify"
	"github.com/caasmo/restinpieces/notify/discord"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/executor"
	"github.com/caasmo/restinpieces/queue/handlers"
	scl "github.com/caasmo/restinpieces/queue/scheduler"
	"github.com/caasmo/restinpieces/router"
	"github.com/caasmo/restinpieces/router/servemux"
	"github.com/caasmo/restinpieces/server"
	"github.com/pelletier/go-toml/v2"
)

// New creates a new App instance and Server with the provided options and age key file path.
// It initializes the core application components like database, router, cache first,
// then loads configuration from the database using the provided age key.
func New(opts ...core.Option) (*core.App, *server.Server, error) {
	app, err := core.NewApp(opts...)
	if err != nil {
		return nil, nil, err
	}

	// Set up temporary bootstrap logger if none was provided before setting the
	// default db based one.
	var withUserLogger = true
	if app.Logger() == nil {
		withUserLogger = false

		app.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})))
	}

	// Setup default router if none was set via options
	if app.Router() == nil {
		if err := SetupDefaultRouter(app); err != nil {
			return nil, nil, err
		}
	}

	// Setup default cache if none was set via options
	if app.Cache() == nil {
		if err := SetupDefaultCache(app); err != nil {
			return nil, nil, err
		}
	}

	// Load config from database
	scope := config.ScopeApplication
	decryptedBytes, err := app.ConfigStore().Latest(scope)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Start with default config
	cfg := config.NewDefaultConfig()

	// Unmarshal TOML into default config
	if err := toml.Unmarshal(decryptedBytes, cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate config
	if err := config.Validate(cfg); err != nil {
		return nil, nil, fmt.Errorf("invalid config: %w", err)
	}

	cfg.Source = "" // Clear source field

	configProvider := config.NewProvider(cfg)
	app.SetConfigProvider(configProvider)

	// Setup default logger if non of user
	logDaemon, err := SetupDefaultLogger(app, configProvider, withUserLogger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to setup logger: %w", err)
	}

	// Setup custom application logic and routes
	route(cfg, app)

	scheduler, err := SetupScheduler(configProvider, app.DbAuth(), app.DbQueue(), app.Logger())
	if err != nil {
		return nil, nil, err
	}

	// Setup default notifier if none was set via options
	if app.Notifier() == nil {
		if err := SetupDefaultNotifier(cfg, app); err != nil {
			return nil, nil, err
		}
	}

	// Prepare the configuration reload function
	reloadFn := config.Reload(app.ConfigStore(), configProvider, app.Logger())

	// Initialize the PreRouter chain with internal middleware
	preRouterHandler := setupPrerouter(app)

	srv := server.NewServer(
		configProvider,
		preRouterHandler,
		app.Logger(),
		reloadFn, // Pass the reload function
	)

	// Register the framework's core daemons
	srv.AddDaemon(scheduler)
	if logDaemon != nil {
		srv.AddDaemon(logDaemon)
	}

	return app, srv, nil
}

// setupPrerouter sets up the internal pre-router middleware chain based on configuration
// and returns the final http.Handler.
// No User Pre-Router Customization:
// we allow disable in config, we do not allow adding, the user can put in normal middleware.
// configure the framework's pre-router features; add your own logic at the route level.
// Framework handles everything before routing; user handles everything after routing
func setupPrerouter(app *core.App) http.Handler {
	logger := app.Logger()
	cfg := app.Config()
	ft := log.NewMessageFormatter().WithComponent("prerouter", "🛠️ ")

	// Start the chain with the application's main router as the base handler.
	// The final handler in the chain will be app.Router().ServeHTTP
	preRouterChain := router.NewChain(app.Router())

	// --- Add Internal Middleware Conditionally (Order Matters!) ---
	// Execution order will be: RequestLog -> BlockIp -> BlockUa -> TLSHeaderSTS -> Maintenance -> app.Router()

	logger.Info(ft.Start("Setting up Prerouter Middleware Chain ..."))

	// 0. Response Recorder Middleware (Added first, runs first)
	recorder := prerouter.NewRecorder(app)
	preRouterChain.WithMiddleware(recorder.Execute)
	logger.Info(ft.Ok("ResponseRecorder middleware added"))

	// 1. Request Logging Middleware (Added second, runs second)
	requestLog := prerouter.NewRequestLog(app)
	preRouterChain.WithMiddleware(requestLog.Execute)
	logger.Info(ft.Ok("RequestLog middleware added"), "activated", cfg.Log.Request.Activated)

	// 2. BlockIp Middleware
	if cfg.BlockIp.Enabled {
		blockIp := prerouter.NewBlockIp(app.Cache(), logger)
		preRouterChain.WithMiddleware(blockIp.Execute)
		logger.Info(ft.Ok("BlockIp middleware enabled"), "activated", cfg.BlockIp.Activated)
	} else {
		logger.Info(ft.Ok("BlockIp middleware not enabled"))
	}

	// 3. Metrics Middleware (only if enabled)
	if cfg.Metrics.Enabled {
		metrics := prerouter.NewMetrics(app)
		preRouterChain.WithMiddleware(metrics.Execute)
		logger.Info(ft.Ok("Metrics middleware enabled"), "activated", cfg.Metrics.Activated)
	} else {
		logger.Info(ft.Ok("Metrics middleware not enabled"))
	}

	// 4. BlockUaList Middleware
	blockUaList := prerouter.NewBlockUaList(app)
	preRouterChain.WithMiddleware(blockUaList.Execute)
	logger.Info(ft.Ok("BlockUaList middleware added"), "activated", cfg.BlockUaList.Activated)

	// 4. TLSHeaderSTS Middleware
	tlsHeaderSTS := prerouter.NewTLSHeaderSTS()
	preRouterChain.WithMiddleware(tlsHeaderSTS.Execute)
	logger.Info(ft.Ok("TLSHeaderSTS middleware added"), "tls_enabled", cfg.Server.EnableTLS)

	// 5. Maintenance Middleware
	maintenance := prerouter.NewMaintenance(app)
	preRouterChain.WithMiddleware(maintenance.Execute)
	logger.Info(ft.Ok("Maintenance middleware added"), "activated", cfg.Maintenance.Activated)

	// 6. BlockRequestBody Middleware
	blockRequestBody := prerouter.NewBlockRequestBody(app)
	preRouterChain.WithMiddleware(blockRequestBody.Execute)
	logger.Info(ft.Ok("BlockRequestBody middleware added"), "activated", cfg.BlockRequestBody.Activated)

	// --- Finalize the PreRouter ---
	preRouterHandler := preRouterChain.Handler()
	logger.Info(ft.Complete("Prerouter Middleware Chain Setup complete"))

	return preRouterHandler
}

// SetupScheduler initializes the job scheduler and its handlers.
// dbAcme parameter removed.
func SetupDefaultRouter(app *core.App) error {
	r := servemux.New()
	app.SetRouter(r)
	return nil
}

func SetupScheduler(configProvider *config.Provider, dbAuth db.DbAuth, dbQueue db.DbQueue, logger *slog.Logger) (*scl.Scheduler, error) {
	ft := log.NewMessageFormatter().WithComponent("scheduler", "🛠️ ")
	logger.Info(ft.Start("Setting up scheduler..."))

	hdls := make(map[string]executor.JobHandler)

	cfg := configProvider.Get()

	// Setup mailer only if SMTP is configured in the current config
	if (cfg.Smtp != config.Smtp{}) {
		mailer, err := mail.New(configProvider)
		if err != nil {
			logger.Error(ft.Fail("failed to create mailer"), "error", err)
			// Decide if this is fatal. If mailing is optional, maybe just log and continue without mail handlers?
			// For now, let's treat it as fatal if configured but failing.
			os.Exit(1) // Or return err
		}

		emailVerificationHandler := handlers.NewEmailVerificationHandler(dbAuth, configProvider, mailer)
		hdls[queue.JobTypeEmailVerification] = emailVerificationHandler
		logger.Info(ft.Ok("registered email verification handler"))

		passwordResetHandler := handlers.NewPasswordResetHandler(dbAuth, configProvider, mailer)
		hdls[queue.JobTypePasswordReset] = passwordResetHandler
		logger.Info(ft.Ok("registered password reset handler"))

		emailChangeHandler := handlers.NewEmailChangeHandler(dbAuth, configProvider, mailer)
		hdls[queue.JobTypeEmailChange] = emailChangeHandler
		logger.Info(ft.Ok("registered email change handler"))
	}

	// ACME handler registration removed.

	scheduler := scl.NewScheduler(configProvider, dbQueue, executor.NewExecutor(hdls), logger)
	logger.Info(ft.Complete("scheduler setup complete"), "handlers_registered", len(hdls))
	return scheduler, nil
}

var DefaultLoggerOptions = &slog.HandlerOptions{
	Level: slog.LevelDebug,
	ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	},
}

func SetupDefaultCache(app *core.App) error {
	ft := log.NewMessageFormatter().WithComponent("cache", "🛠️")
	app.Logger().Info(ft.Start("Initializing Ristretto cache..."))

	cacheInstance, err := ristretto.New[any]() // Explicit string keys and interface{} values
	if err != nil {
		app.Logger().Error(ft.Fail("Ristretto cache initialization failed"), "error", err)
		return fmt.Errorf("failed to initialize Ristretto cache: %w", err)
	}
	app.SetCache(cacheInstance)

	app.Logger().Info(ft.Complete("Ristretto cache initialized successfully"))
	return nil
}

// SetupDefaultLogger initializes the logger daemon and batch handler
// withUserLogger indicates if the app already had a logger configured
func SetupDefaultLogger(app *core.App, configProvider *config.Provider, withUserLogger bool) (*log.Daemon, error) {
	if withUserLogger {
		return nil, nil
	}

	cfg := configProvider.Get()
	logDbPath := cfg.Log.Batch.DbPath
	if logDbPath == "" {
		return nil, fmt.Errorf("logger daemon: database path (LoggerBatch.DbPath) is not configured")
	}

	logDb, err := zombiezen.NewConn(logDbPath)
	if err != nil {
		return nil, fmt.Errorf("logger daemon: failed to open database at %s: %w", logDbPath, err)
	}

	logDaemon, err := log.New(configProvider, app.Logger(), logDb)
	if err != nil {
		return nil, fmt.Errorf("failed to create log daemon: %w", err)
	}

	// Create batch handler with daemon's channel and context
	recordChan, daemonCtx := logDaemon.Chan()
	batchHandler := log.NewBatchHandler(
		configProvider,
		recordChan,
		daemonCtx,
	)

	app.SetLogger(slog.New(batchHandler))

	return logDaemon, nil
}

func SetupDefaultNotifier(cfg *config.Config, app *core.App) error {
	if cfg.Notifier.Discord.Activated {
		discordNotifier, err := discord.New(cfg.Notifier.Discord, app.Logger())
		if err != nil {
			return fmt.Errorf("failed to initialize Discord notifier: %w", err)
		}
		app.SetNotifier(discordNotifier)
	} else {
		app.SetNotifier(notify.NewNilNotifier())
	}
	return nil
}
