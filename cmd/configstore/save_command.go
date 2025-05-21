package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/caasmo/restinpieces/config"
)

func handleSaveCommand(secureStore config.SecureStore, scope string, filename string) {
	if scope == "" {
		scope = config.ScopeApplication
	}

	fileData, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read file '%s': %v\n", filename, err)
		os.Exit(1)
	}

	description := fmt.Sprintf("Inserted from file: %s", filepath.Base(filename))
	format := "toml" // Default format, could be detected from filename if needed

	err = secureStore.Save(scope, fileData, format, description)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save config to database: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully saved file '%s' to scope '%s' in database\n", filename, scope)
}
