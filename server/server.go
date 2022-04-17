package server

import (
	"net/http"
	"time"
	"github.com/caasmo/restinpieces/router"
)

const (
	ReadTimeout =       2 * time.Second
	ReadHeaderTimeout = 2 * time.Second
	WriteTimeout =      3 * time.Second
	IdleTimeout =       1 * time.Minute
)

func New(addr string, r router.Router) *http.Server {
	return &http.Server{
		Addr:              addr,
        Handler:           r,
		ReadTimeout:       ReadTimeout,
		ReadHeaderTimeout: ReadHeaderTimeout,
		WriteTimeout:      WriteTimeout,
		IdleTimeout:       IdleTimeout,
	}
}

