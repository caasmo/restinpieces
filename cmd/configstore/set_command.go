package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml"
)

func handleSetCommand(
	logger *slog.Logger,
	secureCfg config.SecureStore,
	scope string,
	format string,
	defaultDescription string,
	cmdArgs []string) {

	if len(cmdArgs) < 2 {
		logger.Error("missing path or value for 'set' command")
		fmt.Fprintf(os.Stderr, "Usage: configstore ... set <path> <value>\n")
		fmt.Fprintf(os.Stderr, "Set the value at the given TOML path.\n")
		fmt.Fprintf(os.Stderr, "Prefix <value> with '@' (e.g., @./file.txt) to load value from a file.\n")
		os.Exit(1)
	}

	configPath := cmdArgs[0]
	rawValue := cmdArgs[1]

	logger.Info("retrieving latest configuration for 'set' command", "scope", scope)
	decryptedData, err := secureCfg.Latest(scope)
	if err != nil {
		logger.Error("failed to retrieve latest config via SecureStore", "scope", scope, "error", err)
		os.Exit(1)
	}

	tree, err := toml.LoadBytes(decryptedData)
	if err != nil {
		logger.Error("failed to load config data into TOML tree", "scope", scope, "error", err)
		os.Exit(1)
	}

	var valueToSet interface{}
	if strings.HasPrefix(rawValue, "@") {
		filePath := strings.TrimPrefix(rawValue, "@")
		logger.Info("reading value from file", "path", filePath)
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			logger.Error("failed to read value file", "path", filePath, "error", err)
			os.Exit(1)
		}
		valueToSet = string(fileContent)
	} else {
		valueToSet = rawValue
	}

	keyExists := tree.Has(configPath)
	logger.Info("Checking key existence", "path", configPath, "exists", keyExists)

	if !keyExists {
		logger.Error("configuration path does not exist in the TOML structure", "path", configPath)
		os.Exit(1)
	}

	logger.Info("setting configuration value", "path", configPath)
	tree.Set(configPath, valueToSet)

	updatedTomlString, err := tree.ToTomlString()
	if err != nil {
		logger.Error("failed to marshal updated TOML tree to string", "error", err)
		os.Exit(1)
	}
	updatedConfigData := []byte(updatedTomlString)

	description := defaultDescription
	if description == "" {
		description = fmt.Sprintf("Updated field '%s'", configPath)
	}

	logger.Info("saving updated configuration", "scope", scope, "format", format)
	err = secureCfg.Save(scope, updatedConfigData, format, description)
	if err != nil {
		logger.Error("failed to save updated config via SecureStore", "error", err)
		os.Exit(1)
	}

	logger.Info("successfully updated and saved configuration",
		"scope", scope,
		"format", format,
		"path", configPath,
		"description", description)
}
