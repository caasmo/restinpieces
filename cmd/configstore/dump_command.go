package main

import (
	"fmt"
	"os"
)

func handleDumpCommand(secureStore SecureStore, scope string) {
	decryptedData, err := secureStore.Latest(scope)
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
