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
// It initializes the core application components like database, router, cache, etc.
func New(dbfile string, opts ...core.Option) (*core.App, *server.Server, error) {
	// Load initial configuration
	cfg, err := config.Load(dbfile)
	if err != nil {
		slog.Error("failed to load initial config", "error", err)
		// TODO
		return nil, nil, err // Corrected to return (nil, nil, err)
	}

	configProvider := config.NewProvider(cfg)

	// Initialize the core App with provided options and config provider
	// Default options can be added here if needed before user options
	allOpts := []core.Option{core.WithConfigProvider(configProvider)}
	allOpts = append(allOpts, opts...) // Append user-provided options

	app, err := core.NewApp(allOpts...)
	if err != nil {
		slog.Error("failed to initialize core app", "error", err)
		return nil, nil, err
	}

	// Create the Proxy instance, passing the app
	px := proxy.NewProxy(app)

	// Setup custom application logic and routes
	cApp := custom.NewApp(app)
	route(cfg, app, cApp) // Assuming route function exists and is correctly defined elsewhere

	// Setup the scheduler
	scheduler, err := SetupScheduler(configProvider, app.Db(), app.Logger())
	if err != nil {
		// Clean up app resources if scheduler setup fails
		app.Close()
		slog.Error("failed to setup scheduler", "error", err)
		return nil, nil, err
	}

	// Create the server instance
	srv := server.NewServer(configProvider, px, scheduler, app.Logger())
	// app.Logger().Info("Starting server in verbose mode") // Keep commented out or remove

	// Return the initialized app and server
	return app, srv, nil
}

// SetupApp is now redundant as its logic is integrated into New.
// It can be removed or kept if used elsewhere.
// For now, keeping it but commenting out the core.NewApp call as it's done in New.
//func SetupApp(configProvider *config.Provider) (*core.App, *proxy.Proxy, error) {
//	// This function might need refactoring or removal if New handles all setup.
//	// The core.NewApp call is now primarily handled within the New function.
//	// If this function is still needed, it should likely receive an already initialized app
//	// or focus on setting up only the proxy.
//
//	cfg := configProvider.Get()
//
//	// Example: If SetupApp is only responsible for creating the proxy now:
//	// Assuming 'app' is passed in or retrieved differently.
//	// For demonstration, let's assume it needs to create a dummy app for the proxy,
//	// which isn't ideal. Refactoring is recommended.
//	// This part needs clarification on the intended role of SetupApp going forward.
//	// For now, returning nil to avoid compilation errors, but this needs fixing.
//
//	// Placeholder: Create a dummy app instance for the proxy if needed,
//	// but ideally, the app instance should come from the `New` function.
//	// This section requires clarification on the intended flow.
//	// app, err := core.NewApp(...) // This was moved to New
//
//	// Let's assume for now SetupApp is no longer responsible for creating the core app.
//	// It might receive the app as an argument or focus solely on proxy setup.
//	// Returning nil temporarily.
//	var app *core.App // Placeholder
//	var px *proxy.Proxy // Placeholder
//
//	// If the intention is just to create the proxy for an existing app:
//	// px := proxy.NewProxy(app) // 'app' would need to be passed in or available
//
//	return app, px, nil // Needs proper implementation based on refactored role
//}

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

