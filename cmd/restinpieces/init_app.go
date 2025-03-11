package main

import (
	"github.com/caasmo/restinpieces/app"
	// TOD0 problem cgo compile check?
	"github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db/crawshaw"
	"github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/router/httprouter"
	"github.com/caasmo/restinpieces/router/servemux"
)

func WithDBCrawshaw(dbPath string) app.Option {
	db, _ := crawshaw.New(dbPath)
	// TODO erro log fatal

	return app.WithDB(db)
}

func WithDBZombiezen() app.Option {

	db, _ := zombiezen.New("bench.db")
	// TODO erro log fatal

	return app.WithDB(db)
}

func WithRouterServeMux() app.Option {
	r := servemux.New()
	return app.WithRouter(r)
}

func WithRouterHttprouter() app.Option {
	r := httprouter.New()
	return app.WithRouter(r)
}

func WithCacheRistretto() app.Option {
	cache, _ := ristretto.New()
	// TODO fatal
	return app.WithCache(cache)

}

func initApp(cfg *config.Config) (*app.App, error) {
	return app.New(
		WithDBCrawshaw(cfg.DBFile),
		WithRouterServeMux(),
		WithCacheRistretto(),
		app.WithConfig(cfg),
	)
}
