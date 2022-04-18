package main

import (
	"log"
	"net/http"
    "os"
    "os/signal"
    "syscall"
    "context"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/app"
	"github.com/caasmo/restinpieces/server"
	router "github.com/caasmo/restinpieces/router/httprouter"
	cacheRistretto "github.com/caasmo/restinpieces/cache/ristretto"
	"github.com/justinas/alice"
)

func main() {

    // db
	db, err := db.New("bench.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

    // router
	rp := router.NewParamGeter()

    // cache
    cache, err := cacheRistretto.New()

    if err != nil {
        panic(err)
    }

	ap := app.New(db, rp, cache)

	commonMiddleware := alice.New(ap.Logger)

	router := router.New()
	router.Get("/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	router.Get("/", commonMiddleware.ThenFunc(ap.Index))
	router.Get("/example/sqlite/read/randompk",http.HandlerFunc(ap.ExampleSqliteReadRandom))
	router.Get("/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	//router.Get("/example/ristretto/writeread/:value", http.HandlerFunc(ap.ExampleRistrettoWriteRead))
	router.Get("/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	router.Get("/benchmark/sqlite/ratio/:ratio/read/:reads",http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	router.Get("/benchmark/sqlite/pool/ratio/:ratio/read/:reads", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
    // This is an example of init function 
	router.Get("/benchmark/ristretto/read", ap.BenchmarkRistrettoRead())
	router.Get("/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
    srv := server.New(":8080", router)
	//log.Fatal(http.ListenAndServe(":8080", router))

    // move most to server
	go func() {
		// always returns error. ErrServerClosed on graceful close
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
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

    //gracefullCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
    //defer cancelShutdown()

    //if err := httpServer.Shutdown(gracefullCtx); err != nil {
    //    log.Printf("shutdown error: %v\n", err)
    //    defer os.Exit(1)
    //    return
    //} else {
    //    log.Printf("gracefully stopped\n")
    //}

    //defer os.Exit(0)
    os.Exit(0)
}
