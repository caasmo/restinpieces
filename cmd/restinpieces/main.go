package main

import (
	"flag"
	"github.com/caasmo/restinpieces/server"
	"log/slog"
	"os"
)

func main() {
	cfg := &config.Config{
		JwtSecret:     []byte("test_secret_32_bytes_long_xxxxxx"), // 32-byte secret
		TokenDuration: 15 * time.Minute,
		DBFile:        "bench.db",
	}

	dbfile := flag.String("dbfile", cfg.DBFile, "SQLite database file path")
	flag.Parse()
	cfg.DBFile = *dbfile

	ap, err := initApp(cfg)
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	defer ap.Close()

	route(ap)

	server.Run(":8080", ap.Router())
}
