package main

import (
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml"
)

func handleInitCommand(secureStore config.SecureStore, scopeName string) {
	if scopeName == "" {
		scopeName = config.ScopeApplication
	}

	defaultConfig := config.NewDefaultConfig()
	tomlBytes, err := toml.Marshal(defaultConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal default config to TOML: %v\n", err)
		os.Exit(1)
	}

	err = secureStore.Save(scopeName, tomlBytes, "toml", "Initial default configuration")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save default config for scope '%s': %v\n", scopeName, err)
		os.Exit(1)
	}

	fmt.Printf("Successfully saved default configuration for scope '%s'\n", scopeName)
}
