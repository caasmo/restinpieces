package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/pelletier/go-toml/v2"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"zombiezen.com/go/sqlite/sqlitex"
)

var (
	ErrApplySQL = errors.New("failed to apply SQL")
)

// handleAppCreateCommand is the command-level wrapper that executes the core app creation logic.
func handleAppCreateCommand(secureStore config.SecureStore, pool *sqlitex.Pool, dbPath string, ui UI) error {
	return createApplication(ui, secureStore, pool, dbPath)
}

// createApplication contains the testable core logic for creating and configuring the application.
func createApplication(ui UI, secureStore config.SecureStore, pool *sqlitex.Pool, dbPath string) error {
	if err := applyAppSchema(ui, pool); err != nil {
		return err
	}

	// Generate Default Config Struct
	defaultCfg := config.NewDefaultConfig()
	defaultCfg.Jwt.AuthSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)
	defaultCfg.Jwt.PasswordResetSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)
	defaultCfg.Jwt.EmailChangeOtpSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)
	defaultCfg.Jwt.VerificationEmailOtpSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)
	defaultCfg.Jwt.Oauth2StateSecret = crypto.RandomString(32, crypto.AlphanumericAlphabet)

	// Marshal Config to TOML
	tomlBytes, err := toml.Marshal(defaultCfg)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal default config to TOML: %w", ErrConfigMarshal, err)
	}

	// Save Encrypted Config into DB via SecureConfig
	if err := saveConfig(ui, secureStore, tomlBytes); err != nil {
		return err // Error is already wrapped by saveConfig
	}

	if _, err := fmt.Fprintf(ui.Err, "Application database created and configured successfully: %s\n", dbPath); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	return nil
}

func applyAppSchema(ui UI, pool *sqlitex.Pool) error {
	conn, err := pool.Take(context.Background())
	if err != nil {
		return fmt.Errorf("%w: for sql: %w", ErrDbConnection, err)
	}
	defer pool.Put(conn)

	if _, err := fmt.Fprintln(ui.Err, "Applying app schema..."); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	if err := applySQL(conn, "app"); err != nil {
		return fmt.Errorf("%w: sql process failed: %w", ErrApplySQL, err)
	}

	return nil
}

func saveConfig(ui UI, secureStore config.SecureStore, configData []byte) error {
	if _, err := fmt.Fprintln(ui.Err, "Saving initial configuration..."); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}
	err := secureStore.Save(
		config.ScopeApplication,
		configData,
		"toml",
		"Initial default configuration",
	)
	if err != nil {
		return fmt.Errorf("%w: failed to save initial config: %w", ErrSecureStoreSave, err)
	}
	return nil
}
