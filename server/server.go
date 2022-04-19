package server

import (
	"context"
	"github.com/caasmo/restinpieces/router"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	ReadTimeout       = 2 * time.Second
	ReadHeaderTimeout = 2 * time.Second
	WriteTimeout      = 3 * time.Second
	IdleTimeout       = 1 * time.Minute
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

func Run(addr string, r router.Router) {

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadTimeout:       ReadTimeout,
		ReadHeaderTimeout: ReadHeaderTimeout,
		WriteTimeout:      WriteTimeout,
		IdleTimeout:       IdleTimeout,
	}

	// move most to server
	go func() {
		// always returns error. ErrServerClosed on graceful close
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			if err == http.ErrServerClosed {
				log.Print("ErrServerClosed, gratefull")
				return
			}

			// unexpected error. port in use?
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGHUP,  // kill -SIGHUP XXXX
		syscall.SIGINT,  // kill -SIGINT XXXX or Ctrl+c
		syscall.SIGQUIT, // kill -SIGQUIT XXXX
	)

	<-ctx.Done()

	// Reset signals default behavior, similar to signal.Reset
	stop()
	log.Print("os.Interrupt - shutting down...\n")

	gracefullCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(gracefullCtx); err != nil {
		log.Printf("shutdown error: %v\n", err)
		defer os.Exit(1)
		return
	} else {
		log.Printf("gracefully stopped\n")
	}

	defer os.Exit(0)
}
