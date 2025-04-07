package restinpieces

import (
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	r "github.com/caasmo/restinpieces/router"
	"io/fs"
	"net/http"

	// custom handlers and middleware
	"github.com/caasmo/restinpieces/custom"
)

func route(cfg *config.Config, ap *core.App, cAp *custom.App) {
	// Serve static files from configured public directory
	//fs := http.FileServer(http.Dir(cfg.PublicDir))
	//ap.Router().Handle("/", fs)
	//ap.Router().Handle("/assets/", http.StripPrefix("/assets/", fs))

	// --- file server ---
	subFS, err := fs.Sub(EmbeddedAssets, cfg.PublicDir)
	if err != nil {
		// TODO
		panic("failed to create sub filesystem: " + err.Error())
	}

	ffs := http.FileServerFS(subFS)
	ap.Router().Register(
		r.NewRoute("/").WithHandler(ffs).WithMiddleware(
			core.StaticHeadersMiddleware,
			core.GzipMiddleware(subFS),
		),
	)

	// --- api core routes  ---
	ap.Router().Register(
		r.NewRoute("/favicon.ico").WithHandlerFunc(core.FaviconHandler),

		//TODO
		r.NewRoute(cfg.Endpoints.ListEndpoints).WithHandlerFunc(ap.ListEndpointsHandler),

		r.NewRoute(cfg.Endpoints.RefreshAuth).WithHandlerFunc(ap.RefreshAuthHandler),
		r.NewRoute(cfg.Endpoints.AuthWithPassword).WithHandlerFunc(ap.AuthWithPasswordHandler),
		r.NewRoute(cfg.Endpoints.AuthWithOAuth2).WithHandlerFunc(ap.AuthWithOAuth2Handler),
		r.NewRoute(cfg.Endpoints.RequestEmailVerification).WithHandlerFunc(ap.RequestEmailVerificationHandler),
		r.NewRoute(cfg.Endpoints.RegisterWithPassword).WithHandlerFunc(ap.RegisterWithPasswordHandler),
		r.NewRoute(cfg.Endpoints.ListOAuth2Providers).WithHandlerFunc(ap.ListOAuth2ProvidersHandler),
		r.NewRoute(cfg.Endpoints.ConfirmEmailVerification).WithHandlerFunc(ap.ConfirmEmailVerificationHandler),
		r.NewRoute(cfg.Endpoints.RequestPasswordReset).WithHandlerFunc(ap.RequestPasswordResetHandler),
		r.NewRoute(cfg.Endpoints.ConfirmPasswordReset).WithHandlerFunc(ap.ConfirmPasswordResetHandler),
		r.NewRoute(cfg.Endpoints.RequestEmailChange).WithHandlerFunc(ap.RequestEmailChangeHandler),
		r.NewRoute(cfg.Endpoints.ConfirmEmailChange).WithHandlerFunc(ap.ConfirmEmailChangeHandler),

		// --- custom routes  ---

		r.NewRoute("GET /custom").WithHandlerFunc(cAp.Index),
		// Test route for IP blocking functionality
		//r.NewRoute("GET /blocktest").WithHandlerFunc(cAp.Index).WithMiddleware(ap.BlockMiddleware()),
	)

	//ap.Router().Handle("/api/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	//ap.Router().Handle("/api/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	//ap.Router().Handle("/api/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	//ap.Router().Handle("/api/benchmark/sqlite/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	//ap.Router().Handle("GET /api/benchmark/sqlite/pool/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	//ap.Router().Handle("/api/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	//ap.Router().Handle("/api/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
}

