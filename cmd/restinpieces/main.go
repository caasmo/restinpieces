package main

import (
	"log"
	"net/http"
    "os"
    "os/signal"
    "syscall"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/app"
	"github.com/caasmo/restinpieces/server"
	router "github.com/caasmo/restinpieces/router/httprouter"
	"github.com/justinas/alice"
)

func main() {

	db, err := db.New("bench.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rp := router.NewParamGeter()
	ap := app.New(db, rp)

	commonMiddleware := alice.New(ap.Logger)

	router := router.New()
	router.Get("/admin", commonMiddleware.Append(ap.Auth).ThenFunc(ap.Admin))
	router.Get("/", commonMiddleware.ThenFunc(ap.Index))
	router.Get("/example/sqlite/read/randompk",http.HandlerFunc(ap.ExampleSqliteReadRandom))
	router.Get("/example/sqlite/writeone/:value", http.HandlerFunc(ap.ExampleWriteOne))
	router.Get("/benchmark/baseline", http.HandlerFunc(ap.BenchmarkBaseline))
	router.Get("/benchmark/sqlite/ratio/:ratio/read/:reads",http.HandlerFunc(ap.BenchmarkSqliteRWRatio))
	router.Get("/benchmark/sqlite/pool/ratio/:ratio/read/:reads", http.HandlerFunc(ap.BenchmarkSqliteRWRatioPool))
	router.Get("/teas/:id", commonMiddleware.ThenFunc(ap.Tea))
    srv := server.New(":8080", router)
	//log.Fatal(http.ListenAndServe(":8080", router))

	go func() {
		// always returns error. ErrServerClosed on graceful close
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

    signalChan := make(chan os.Signal, 1)
    signal.Notify(
		signalChan,
		syscall.SIGHUP,  // kill -SIGHUP XXXX
		syscall.SIGINT,  // kill -SIGINT XXXX or Ctrl+c
		syscall.SIGQUIT, // kill -SIGQUIT XXXX
	)

	<-signalChan

	//log.Print("os.Interrupt - shutting down...\n")
	log.Fatal("os.Kill - terminating...\n")
}
