package main

import (
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
)

func handleSaveCommand(secureStore config.SecureStore, scope string, filename string) {
	if scope == "" {
		scope = config.ScopeApplication
	}

	decryptedData, err := secureStore.Latest(scope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to retrieve latest config for scope '%s': %v\n", scope, err)
		os.Exit(1)
	}

	err = os.WriteFile(filename, decryptedData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to write config to file '%s': %v\n", filename, err)
		os.Exit(1)
	}

	fmt.Printf("Successfully saved config for scope '%s' to file '%s'\n", scope, filename)
}
