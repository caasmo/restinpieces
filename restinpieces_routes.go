package restinpieces

import (
	"net/http"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
)

func route(cfg *config.Config, ap *core.App) {

	// --- api core routes  ---
	// Favicon - Note: core.FaviconHandler is already an http.HandlerFunc
	ap.Router().Handle("/favicon.ico", core.NewChain(http.HandlerFunc(core.FaviconHandler)).Handler())

	// TODO: List Endpoints
	ap.Router().Handle(cfg.Endpoints.ListEndpoints, core.NewChain(http.HandlerFunc(ap.ListEndpointsHandler)).Handler())

	// Auth Routes
	ap.Router().Handle(cfg.Endpoints.RefreshAuth, core.NewChain(http.HandlerFunc(ap.RefreshAuthHandler)).Handler())
	ap.Router().Handle(cfg.Endpoints.AuthWithPassword, core.NewChain(http.HandlerFunc(ap.AuthWithPasswordHandler)).Handler())
	ap.Router().Handle(cfg.Endpoints.AuthWithOAuth2, core.NewChain(http.HandlerFunc(ap.AuthWithOAuth2Handler)).Handler())
	ap.Router().Handle(cfg.Endpoints.RegisterWithPassword, core.NewChain(http.HandlerFunc(ap.RegisterWithPasswordHandler)).Handler())
	ap.Router().Handle(cfg.Endpoints.ListOAuth2Providers, core.NewChain(http.HandlerFunc(ap.ListOAuth2ProvidersHandler)).Handler())

	// Email Verification
	ap.Router().Handle(cfg.Endpoints.RequestEmailVerification, core.NewChain(http.HandlerFunc(ap.RequestEmailVerificationHandler)).Handler())
	ap.Router().Handle(cfg.Endpoints.ConfirmEmailVerification, core.NewChain(http.HandlerFunc(ap.ConfirmEmailVerificationHandler)).Handler())

	// Password Reset
	ap.Router().Handle(cfg.Endpoints.RequestPasswordReset, core.NewChain(http.HandlerFunc(ap.RequestPasswordResetHandler)).Handler())
	ap.Router().Handle(cfg.Endpoints.ConfirmPasswordReset, core.NewChain(http.HandlerFunc(ap.ConfirmPasswordResetHandler)).Handler())

	// Email Change
	ap.Router().Handle(cfg.Endpoints.RequestEmailChange, core.NewChain(http.HandlerFunc(ap.RequestEmailChangeHandler)).Handler())
	ap.Router().Handle(cfg.Endpoints.ConfirmEmailChange, core.NewChain(http.HandlerFunc(ap.ConfirmEmailChangeHandler)).Handler())

	// --- Example/Benchmark Routes (keep commented for now) ---
	// ap.Router().Handle("/api/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	// ap.Router().Handle("/api/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	//ap.Router().Handle("/api/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	//ap.Router().Handle("/api/benchmark/sqlite/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	//ap.Router().Handle("GET /api/benchmark/sqlite/pool/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	//ap.Router().Handle("/api/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	//ap.Router().Handle("/api/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
}
