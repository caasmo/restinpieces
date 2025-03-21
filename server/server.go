package server

import (
	"context"
	"github.com/caasmo/restinpieces/router"
	"log"
	"net/http"
	"os"
	"os/signal"
	"golang.org/x/sync/errgroup"
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

	// Start HTTP server
	serverError := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("ListenAndServe(): %v", err)
			serverError <- err
		}
	}()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGHUP,  // kill -SIGHUP XXXX
		syscall.SIGINT,  // kill -SIGINT XXXX or Ctrl+c
		syscall.SIGQUIT, // kill -SIGQUIT XXXX
	)

	// Wait for either interrupt signal or server error
	select {
	case <-ctx.Done():
		log.Print("Received shutdown signal - gracefully shutting down...\n")
	case err := <-serverError:
		log.Printf("Server error: %v - initiating shutdown...\n", err)
	}

	// Reset signals default behavior, similar to signal.Reset
	stop()

    // TODO constant
	gracefulCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()

	// Create a wait group for shutdown tasks
	shutdownGroup, _ := errgroup.WithContext(gracefulCtx)
	
	// Shutdown HTTP server in a goroutine
	shutdownGroup.Go(func() error {
		log.Println("Shutting down HTTP server...")
		if err := srv.Shutdown(gracefulCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v\n", err)
			return err
		}
		log.Printf("HTTP server stopped gracefully\n")
		return nil
	})
	
	// Shutdown scheduler in a goroutine, passing the graceful context
	//shutdownGroup.Go(func() error {
	//	log.Println("Shutting down scheduler...")
	//	if err := scheduler.StopWithContext(gracefulCtx); err != nil {
	//		log.Printf("Scheduler shutdown error: %v\n", err)
	//		return err
	//	}
	//	log.Printf("Scheduler stopped gracefully\n")
	//	return nil
	//})
	
	// Wait for all shutdown tasks to complete
	if err := shutdownGroup.Wait(); err != nil {
		log.Printf("Error during shutdown: %v\n", err)
		os.Exit(1)
	}
	
	log.Printf("All systems stopped gracefully\n")
	os.Exit(0)

}
