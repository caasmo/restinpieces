package main

import (
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
)

func handleDumpCommand(secureStore config.SecureStore, scope string) {
	if scope == "" {
		scope = config.ScopeApplication
	}
	decryptedData, _, err := secureStore.Get(scope, 0) // generation 0 = latest
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to retrieve latest config for scope '%s': %v\n", scope, err)
		os.Exit(1)
	}

	_, err = os.Stdout.Write(decryptedData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to write config to stdout: %v\n", err)
		os.Exit(1)
	}
}
