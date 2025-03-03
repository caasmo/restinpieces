package main

import (
	"github.com/caasmo/restinpieces/app"
	cacheRistretto "github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/router/servemux"
	//"github.com/caasmo/restinpieces/router/httprouter"
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

	return app.New(db, servemux.New(), app.WithCache(cache)), nil
}

func main() {

	ap, err := initApp()
	if err != nil {
		//log
		os.Exit(1)
	}

	defer ap.Close()

	route(ap)

	server.Run(":8080", ap.Router())
}
