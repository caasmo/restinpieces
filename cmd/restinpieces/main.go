package main

import (
	"github.com/caasmo/restinpieces/server"
	"log/slog"
	"os"
)

func main() {

	ap, err := initApp()
	if err != nil {
		slog.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	defer ap.Close()

	route(ap)

	server.Run(":8080", ap.Router())
}
