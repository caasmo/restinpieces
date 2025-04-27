package main

import (
	"bytes"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	dbz "github.com/caasmo/restinpieces/db/zombiezen"
	toml "github.com/pelletier/go-toml/v2"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ageIdentityPathFlag := flag.String("age-key", "", "Path to the age identity file (private key 'AGE-SECRET-KEY-1...') (required)")
	dbPathFlag := flag.String("db", "", "Path to the SQLite database file (required)")
	scopeFlag := flag.String("scope", config.ScopeApplication, "Scope for the configuration (e.g., 'application', 'plugin_x')")
	formatFlag := flag.String("format", "toml", "Format of the configuration file (e.g., 'toml', 'json')")
	descFlag := flag.String("desc", "", "Optional description for this configuration version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -age-key <identity-file> -db <db-file> [options] set <path> <value>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Edits a configuration value stored securely in the database.\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  set <path> <value>   Set the value at the given TOML path.\n")
		fmt.Fprintf(os.Stderr, "                       Prefix <value> with '@' (e.g., @./file.txt) to load value from a file.\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *ageIdentityPathFlag == "" || *dbPathFlag == "" {
		logger.Error("missing required flags: -age-key and -db")
		flag.Usage()
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) < 3 {
		logger.Error("missing command, path, or value")
		flag.Usage()
		os.Exit(1)
	}

	command := args[0]
	configPath := args[1]
	rawValue := args[2]

	if command != "set" {
		logger.Error("invalid command", "command", command)
		flag.Usage()
		os.Exit(1)
	}

	logger.Info("creating sqlite database pool", "path", *dbPathFlag)
	pool, err := restinpieces.NewZombiezenPool(*dbPathFlag)
	if err != nil {
		logger.Error("failed to create database pool", "db_path", *dbPathFlag, "error", err)
		os.Exit(1)
	}
	defer func() {
		logger.Info("closing database pool")
		if err := pool.Close(); err != nil {
			logger.Error("error closing database pool", "error", err)
		}
	}()

	dbImpl, err := dbz.New(pool)
	if err != nil {
		logger.Error("failed to instantiate zombiezen db from pool", "error", err)
		os.Exit(1)
	}

	secureCfg, err := config.NewSecureConfigAge(dbImpl, *ageIdentityPathFlag, logger)
	if err != nil {
		logger.Error("failed to instantiate secure config (age)", "age_key_path", *ageIdentityPathFlag, "error", err)
		os.Exit(1)
	}

	logger.Info("retrieving latest configuration", "scope", *scopeFlag)
	decryptedData, err := secureCfg.Latest(*scopeFlag)
	if err != nil {
		logger.Error("failed to retrieve latest config via SecureConfig", "scope", *scopeFlag, "error", err)
		os.Exit(1)
	}

	tree, err := toml.LoadBytes(decryptedData)
	if err != nil {
		logger.Error("failed to load config data into TOML tree", "scope", *scopeFlag, "error", err)
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

	logger.Info("setting configuration value", "path", configPath)
	err = tree.Set(configPath, valueToSet)
	if err != nil {
		logger.Error("failed to set value in TOML tree", "path", configPath, "error", err)
		os.Exit(1)
	}

	var updatedConfigData bytes.Buffer
	encoder := toml.NewEncoder(&updatedConfigData)
	encoder.SetIndentTables(true) // Optional: Keep indentation for readability
	err = encoder.Encode(tree)
	if err != nil {
		logger.Error("failed to marshal updated TOML tree", "error", err)
		os.Exit(1)
	}

	description := *descFlag
	if description == "" {
		description = fmt.Sprintf("Updated field '%s'", configPath)
	}

	logger.Info("saving updated configuration", "scope", *scopeFlag, "format", *formatFlag)
	err = secureCfg.Save(*scopeFlag, updatedConfigData.Bytes(), *formatFlag, description)
	if err != nil {
		logger.Error("failed to save updated config via SecureConfig", "error", err)
		os.Exit(1)
	}

	logger.Info("successfully updated and saved configuration",
		"scope", *scopeFlag,
		"format", *formatFlag,
		"path", configPath,
		"description", description)
}
