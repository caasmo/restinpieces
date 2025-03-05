package main

import (
	"github.com/caasmo/restinpieces/server"
	"os"
)

func main() {

	ap, err := initApp()
	if err != nil {
		//log
		os.Exit(1)
	}

	defer ap.Close()

	route(ap)

	server.Run(":8080", ap.Router())
}
