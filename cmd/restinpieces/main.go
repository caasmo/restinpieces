package main

import (
	"flag"
	"github.com/caasmo/restinpieces/server"
	"log/slog"
	"os"
)

func main() {
	dbfile := flag.String("dbfile", "bench.db", "SQLite database file path")
	flag.Parse()

	ap, err := initApp(*dbfile)
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	defer ap.Close()

	route(ap)

	server.Run(":8080", ap.Router())
}
