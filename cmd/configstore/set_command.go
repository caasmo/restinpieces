package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml"
)

func handleSetCommand(
	secureCfg config.SecureStore,
	scope string,
	format string,
	defaultDescription string,
	cmdArgs []string) {

	if scope == "" {
		scope = config.ScopeApplication
	}

	if len(cmdArgs) < 2 {
		fmt.Fprintf(os.Stderr, "Error: missing path or value for 'set' command\n")
		fmt.Fprintf(os.Stderr, "Usage: configstore ... set <path> <value>\n")
		fmt.Fprintf(os.Stderr, "Set the value at the given TOML path.\n")
		fmt.Fprintf(os.Stderr, "Prefix <value> with '@' (e.g., @./file.txt) to load value from a file.\n")
		os.Exit(1)
	}

	configPath := cmdArgs[0]
	rawValue := cmdArgs[1]

	decryptedData, err := secureCfg.Latest(scope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to retrieve latest config via SecureStore (scope: %s): %v\n", scope, err)
		os.Exit(1)
	}

	tree, err := toml.LoadBytes(decryptedData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load config data into TOML tree (scope: %s): %v\n", scope, err)
		os.Exit(1)
	}

	var valueToSet interface{}
	if strings.HasPrefix(rawValue, "@") {
		filePath := strings.TrimPrefix(rawValue, "@")
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read value file (path: %s): %v\n", filePath, err)
			os.Exit(1)
		}
		valueToSet = string(fileContent)
	} else {
		valueToSet = rawValue
	}

	keyExists := tree.Has(configPath)

	if !keyExists {
		fmt.Fprintf(os.Stderr, "Error: configuration path does not exist in the TOML structure: %s\n", configPath)
		os.Exit(1)
	}

	tree.Set(configPath, valueToSet)

	updatedTomlString, err := tree.ToTomlString()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal updated TOML tree to string: %v\n", err)
		os.Exit(1)
	}
	updatedConfigData := []byte(updatedTomlString)

	description := defaultDescription
	if description == "" {
		description = fmt.Sprintf("Updated field '%s'", configPath)
	}

	err = secureCfg.Save(scope, updatedConfigData, format, description)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save updated config via SecureStore: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully set '%s' in scope '%s'\n", configPath, scope)
}
