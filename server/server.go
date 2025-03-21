package server

import (
	"context"
	"github.com/caasmo/restinpieces/router"
	"log/slog"
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
			slog.Error("ListenAndServe error", "err", err)
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
		slog.Info("Received shutdown signal - gracefully shutting down")
	case err := <-serverError:
		slog.Error("Server error - initiating shutdown", "err", err)
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
		slog.Info("Shutting down HTTP server")
		if err := srv.Shutdown(gracefulCtx); err != nil {
			slog.Error("HTTP server shutdown error", "err", err)
			return err
		}
		slog.Info("HTTP server stopped gracefully")
		return nil
	})
	
	// Shutdown scheduler in a goroutine, passing the graceful context
	//shutdownGroup.Go(func() error {
	//	log.Println("Shutting down scheduler...")
	//	if err := scheduler.Stop(gracefulCtx); err != nil {
	//		log.Printf("Scheduler shutdown error: %v\n", err)
	//		return err
	//	}
	//	log.Printf("Scheduler stopped gracefully\n")
	//	return nil
	//})
	
	// Wait for all shutdown tasks to complete
	if err := shutdownGroup.Wait(); err != nil {
		slog.Error("Error during shutdown", "err", err)
		os.Exit(1)
	}
	
	slog.Info("All systems stopped gracefully")
	os.Exit(0)

}
