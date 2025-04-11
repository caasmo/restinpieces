package restinpieces

import (
	"net/http"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

func route(cfg *config.Config, ap *core.App) {

	// --- api core routes ---

	// Define all routes within the Register call's map literal

	// Favicon - Note: core.FaviconHandler is already an http.HandlerFunc
	routes["/favicon.ico"] = core.NewChain(http.HandlerFunc(core.FaviconHandler)) // No .Handler()

	// TODO: List Endpoints
	routes[cfg.Endpoints.ListEndpoints] = core.NewChain(http.HandlerFunc(ap.ListEndpointsHandler)) // No .Handler()

	// Auth Routes
	routes[cfg.Endpoints.RefreshAuth] = core.NewChain(http.HandlerFunc(ap.RefreshAuthHandler)) // No .Handler()
	routes[cfg.Endpoints.AuthWithPassword] = core.NewChain(http.HandlerFunc(ap.AuthWithPasswordHandler)) // No .Handler()
	routes[cfg.Endpoints.AuthWithOAuth2] = core.NewChain(http.HandlerFunc(ap.AuthWithOAuth2Handler)) // No .Handler()
	routes[cfg.Endpoints.RegisterWithPassword] = core.NewChain(http.HandlerFunc(ap.RegisterWithPasswordHandler)) // No .Handler()
	routes[cfg.Endpoints.ListOAuth2Providers] = core.NewChain(http.HandlerFunc(ap.ListOAuth2ProvidersHandler)) // No .Handler()

	// Email Verification
	routes[cfg.Endpoints.RequestEmailVerification] = core.NewChain(http.HandlerFunc(ap.RequestEmailVerificationHandler)) // No .Handler()
	routes[cfg.Endpoints.ConfirmEmailVerification] = core.NewChain(http.HandlerFunc(ap.ConfirmEmailVerificationHandler)) // No .Handler()

	// Password Reset
	routes[cfg.Endpoints.RequestPasswordReset] = core.NewChain(http.HandlerFunc(ap.RequestPasswordResetHandler)) // No .Handler()
	routes[cfg.Endpoints.ConfirmPasswordReset] = core.NewChain(http.HandlerFunc(ap.ConfirmPasswordResetHandler)) // No .Handler()

	// Email Change
	routes[cfg.Endpoints.RequestEmailChange] = core.NewChain(http.HandlerFunc(ap.RequestEmailChangeHandler)) // No .Handler()
	routes[cfg.Endpoints.ConfirmEmailChange] = core.NewChain(http.HandlerFunc(ap.ConfirmEmailChangeHandler)) // No .Handler()

	// --- Example/Benchmark Routes (keep commented for now) ---
	// routes["/api/admin"] = core.NewChain(http.HandlerFunc(ap.Admin)).WithMiddleware(ap.Auth) // Example with middleware
	// routes["/api/example/sqlite/writeone/:value"] = core.NewChain(http.HandlerFunc(ap.ExampleWriteOne))
	// routes["/api/benchmark/baseline"] = core.NewChain(http.HandlerFunc(ap.BenchmarkBaseline))
	// routes["/api/benchmark/sqlite/ratio/{ratio}/read/{reads}"] = core.NewChain(http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	// routes["GET /api/benchmark/sqlite/pool/ratio/{ratio}/read/{reads}"] = core.NewChain(http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	// routes["/api/benchmark/ristretto/read"] = core.NewChain(ap.BenchmarkRistrettoRead()) // Assuming this returns http.HandlerFunc
	// routes["/api/teas/:id"] = core.NewChain(http.HandlerFunc(ap.Tea))

	// Register all routes at once using a map literal
	ap.Router().Register(map[string]*core.Chain{
		// Favicon
		"/favicon.ico": core.NewChain(http.HandlerFunc(core.FaviconHandler)),

		// List Endpoints
		cfg.Endpoints.ListEndpoints: core.NewChain(http.HandlerFunc(ap.ListEndpointsHandler)),

		// Auth Routes
		cfg.Endpoints.RefreshAuth:          core.NewChain(http.HandlerFunc(ap.RefreshAuthHandler)),
		cfg.Endpoints.AuthWithPassword:     core.NewChain(http.HandlerFunc(ap.AuthWithPasswordHandler)),
		cfg.Endpoints.AuthWithOAuth2:       core.NewChain(http.HandlerFunc(ap.AuthWithOAuth2Handler)),
		cfg.Endpoints.RegisterWithPassword: core.NewChain(http.HandlerFunc(ap.RegisterWithPasswordHandler)),
		cfg.Endpoints.ListOAuth2Providers:  core.NewChain(http.HandlerFunc(ap.ListOAuth2ProvidersHandler)),

		// Email Verification
		cfg.Endpoints.RequestEmailVerification: core.NewChain(http.HandlerFunc(ap.RequestEmailVerificationHandler)),
		cfg.Endpoints.ConfirmEmailVerification: core.NewChain(http.HandlerFunc(ap.ConfirmEmailVerificationHandler)),

		// Password Reset
		cfg.Endpoints.RequestPasswordReset: core.NewChain(http.HandlerFunc(ap.RequestPasswordResetHandler)),
		cfg.Endpoints.ConfirmPasswordReset: core.NewChain(http.HandlerFunc(ap.ConfirmPasswordResetHandler)),

		// Email Change
		cfg.Endpoints.RequestEmailChange: core.NewChain(http.HandlerFunc(ap.RequestEmailChangeHandler)),
		cfg.Endpoints.ConfirmEmailChange: core.NewChain(http.HandlerFunc(ap.ConfirmEmailChangeHandler)),

		// --- Example/Benchmark Routes (keep commented for now) ---
		// "/api/admin": core.NewChain(http.HandlerFunc(ap.Admin)).WithMiddleware(ap.Auth), // Example with middleware
		// "/api/example/sqlite/writeone/:value": core.NewChain(http.HandlerFunc(ap.ExampleWriteOne)),
		// "/api/benchmark/baseline": core.NewChain(http.HandlerFunc(ap.BenchmarkBaseline)),
		// "/api/benchmark/sqlite/ratio/{ratio}/read/{reads}": core.NewChain(http.HandlerFunc(ap.BenchmarkSqliteRWRatio)),
		// "GET /api/benchmark/sqlite/pool/ratio/{ratio}/read/{reads}": core.NewChain(http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool)),
		// "/api/benchmark/ristretto/read": core.NewChain(ap.BenchmarkRistrettoRead()), // Assuming this returns http.HandlerFunc
		// "/api/teas/:id": core.NewChain(http.HandlerFunc(ap.Tea)),
	})
}
