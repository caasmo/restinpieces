package restinpieces

import (
	"net/http"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	"github.com/caasmo/restinpieces/router"
)

//func WithPreRouterMiddleware(m )  {
//    
//}

func route(cfg *config.Config, ap *core.App) {

	// --- api core routes ---
	ap.Router().Register(map[string]*router.Chain{
		// Favicon
		"/favicon.ico": router.NewChain(http.HandlerFunc(core.FaviconHandler)),

		// List Endpoints
		cfg.Endpoints.ListEndpoints: router.NewChain(http.HandlerFunc(ap.ListEndpointsHandler)),

		// Auth Routes
		cfg.Endpoints.RefreshAuth:          router.NewChain(http.HandlerFunc(ap.RefreshAuthHandler)),
		cfg.Endpoints.AuthWithPassword:     router.NewChain(http.HandlerFunc(ap.AuthWithPasswordHandler)),
		cfg.Endpoints.AuthWithOAuth2:       router.NewChain(http.HandlerFunc(ap.AuthWithOAuth2Handler)),
		cfg.Endpoints.RegisterWithPassword: router.NewChain(http.HandlerFunc(ap.RegisterWithPasswordHandler)),
		cfg.Endpoints.ListOAuth2Providers:  router.NewChain(http.HandlerFunc(ap.ListOAuth2ProvidersHandler)),

		// Email Verification
		cfg.Endpoints.RequestEmailVerification: router.NewChain(http.HandlerFunc(ap.RequestEmailVerificationHandler)),
		cfg.Endpoints.ConfirmEmailVerification: router.NewChain(http.HandlerFunc(ap.ConfirmEmailVerificationHandler)),

		// Password Reset
		cfg.Endpoints.RequestPasswordReset: router.NewChain(http.HandlerFunc(ap.RequestPasswordResetHandler)),
		cfg.Endpoints.ConfirmPasswordReset: router.NewChain(http.HandlerFunc(ap.ConfirmPasswordResetHandler)),

		// Email Change
		cfg.Endpoints.RequestEmailChange: router.NewChain(http.HandlerFunc(ap.RequestEmailChangeHandler)),
		cfg.Endpoints.ConfirmEmailChange: router.NewChain(http.HandlerFunc(ap.ConfirmEmailChangeHandler)),

		// --- Example/Benchmark Routes (keep commented for now) ---
		// "/api/admin": router.NewChain(http.HandlerFunc(ap.Admin)).WithMiddleware(ap.Auth), // Example with middleware
		// "/api/example/sqlite/writeone/:value": router.NewChain(http.HandlerFunc(ap.ExampleWriteOne)),
		// "/api/benchmark/baseline": router.NewChain(http.HandlerFunc(ap.BenchmarkBaseline)),
		// "/api/benchmark/sqlite/ratio/{ratio}/read/{reads}": router.NewChain(http.HandlerFunc(ap.BenchmarkSqliteRWRatio)),
		// "GET /api/benchmark/sqlite/pool/ratio/{ratio}/read/{reads}": router.NewChain(http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool)),
		// "/api/benchmark/ristretto/read": router.NewChain(ap.BenchmarkRistrettoRead()), // Assuming this returns http.HandlerFunc
		// "/api/teas/:id": router.NewChain(http.HandlerFunc(ap.Tea)),
	})
}
