package main

import (
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/server"
)

func main() {

	dbfile := flag.String("dbfile", "bench.db", "SQLite database file path")
	flag.Parse()

	cfg := &config.Config{
		JwtSecret:     []byte("test_secret_32_bytes_long_xxxxxx"), // 32-byte secret
		TokenDuration: 15 * time.Minute,
		DBFile:        *dbfile,
	}

	ap, err := initApp(cfg)
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	defer ap.Close()

	route(ap)

	server.Run(":8080", ap.Router())
}
