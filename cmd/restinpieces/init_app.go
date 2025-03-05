package main

import (
	"github.com/caasmo/restinpieces/app"
	cacheRistretto "github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/db/crawshaw"
	"github.com/caasmo/restinpieces/router/httprouter"
)

func initApp() (*app.App, error) {
	// db
	db, err := crawshaw.New("bench.db")
	if err != nil {
		return nil, err
	}

	// router
	r := httprouter.New()

	// cache
	cache, err := cacheRistretto.New()
	if err != nil {
		return nil, err
	}

	return app.New(
		app.WithDB(db),
		app.WithRouter(r), 
		app.WithCache(cache),
	)
}
