package main

import (
	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/core"
	r "github.com/caasmo/restinpieces/router"
	"io/fs"
	"net/http"

	// custom handlers and middleware
	"github.com/caasmo/restinpieces/custom"
)

func route(cfg *config.Config, ap *core.App, cAp *custom.App) {

	// --- file server ---
	subFS, err := fs.Sub(restinpieces.EmbeddedAssets, cfg.PublicDir)
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

	// --- TODO ---
	commonNewMiddleware := []func(http.Handler) http.Handler{ap.Logger}

	// --- api core routes  ---
	ap.Router().Register(
		//TODO
		r.NewRoute(cfg.Endpoints.ListEndpoints).WithHandlerFunc(ap.ListEndpointsHandler),

		r.NewRoute(cfg.Endpoints.RefreshAuth).WithHandlerFunc(ap.RefreshAuthHandler),
		r.NewRoute(cfg.Endpoints.AuthWithPassword).WithHandlerFunc(ap.AuthWithPasswordHandler),
		r.NewRoute(cfg.Endpoints.AuthWithOAuth2).WithHandlerFunc(ap.AuthWithOAuth2Handler),
		r.NewRoute(cfg.Endpoints.RequestVerification).WithHandlerFunc(ap.RequestVerificationHandler),
		r.NewRoute(cfg.Endpoints.RegisterWithPassword).WithHandlerFunc(ap.RegisterWithPasswordHandler),
		r.NewRoute(cfg.Endpoints.ListOAuth2Providers).WithHandlerFunc(ap.ListOAuth2ProvidersHandler).WithMiddlewareChain(commonNewMiddleware),
		r.NewRoute(cfg.Endpoints.ConfirmVerification).WithHandlerFunc(ap.ConfirmVerificationHandler),

		// --- custom routes  ---

		r.NewRoute("GET /custom").WithHandlerFunc(cAp.Index),
		// Test route for IP blocking functionality
		r.NewRoute("GET /blocktest").WithHandlerFunc(cAp.Index).WithMiddleware(ap.BlockMiddleware()),
	)

	//ap.Router().Handle("/api/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	//ap.Router().Handle("/api/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	//ap.Router().Handle("/api/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	//ap.Router().Handle("/api/benchmark/sqlite/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	//ap.Router().Handle("GET /api/benchmark/sqlite/pool/ratio/{ratio}/read/{reads}", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	//ap.Router().Handle("/api/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	//ap.Router().Handle("/api/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
}
