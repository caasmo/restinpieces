package main

import (
	"github.com/caasmo/restinpieces/app"
	cacheRistretto "github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/db"
	router "github.com/caasmo/restinpieces/router/httprouter"
	"github.com/caasmo/restinpieces/server"
	"os"
)

func initApp() (*app.App, error) {

	// db
	db, err := db.New("bench.db")
	if err != nil {
		return nil, err
	}

	// cache
	cache, err := cacheRistretto.New()
	if err != nil {
		return nil, err
	}

	return app.New(db, router.New(), cache), nil
}

func main() {

	ap, err := initApp()
	if err != nil {
		//log
		os.Exit(1)
	}

	defer ap.Close()

	r := router.New()
	route(r, ap)

	server.Run(":8080", r)
}
