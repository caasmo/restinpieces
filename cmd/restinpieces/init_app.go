package main

import (
	"time"
	"github.com/caasmo/restinpieces/app"
    // TOD0 problem cgo compile check?
	"github.com/caasmo/restinpieces/db/crawshaw"
    "github.com/caasmo/restinpieces/db/zombiezen"
	"github.com/caasmo/restinpieces/router/httprouter"
	"github.com/caasmo/restinpieces/router/servemux"
	"github.com/caasmo/restinpieces/cache/ristretto"
)

func WithDBCrawshaw() app.Option {

	db, _ := crawshaw.New("bench.db")
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

func initApp() (*app.App, error) {

	// Create default config
	cfg := &app.Config{
		JwtSecret:     []byte("the_jwt_secret"), // Get secret from environment
		TokenDuration: 15 * time.Minute,               // 15 minute token duration
	}

	return app.New(
		WithDBCrawshaw(),
		WithRouterServeMux(),
		WithCacheRistretto(),
        app.WithConfig(cfg),
	)
}
