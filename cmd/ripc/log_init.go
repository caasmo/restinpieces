package main

import (
	"errors"
	"fmt"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

var ErrUpdateLogPath = errors.New("failed to update log path")

// handleLogInitCommand is the command-level wrapper. It executes the core logic
// and returns any error to the caller.
func handleLogInitCommand(secureStore config.SecureStore, appDbPath string, logPath string, ui UI) error {
	return logInit(ui, secureStore, appDbPath, logPath)
}

// logInit contains the testable core logic for initializing the log database.
func logInit(ui UI, secureStore config.SecureStore, appDbPath string, logPathArg string) (err error) {
	logDbPath := logPathArg
	if logDbPath == "" {
		logDbPath = appDbPath
	}

	_, err = fmt.Fprintf(ui.Err, "Initializing log database at: %s\n", logDbPath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	_, err = fmt.Fprintln(ui.Err, "Applying log schema...")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	ldb, err := newLogDb(logDbPath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := ldb.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close log db: %w", cerr))
		}
	}()

	err = ldb.createSchemas()
	if err != nil {
		return err
	}

	err = updateLogPathInConfig(secureStore, logDbPath)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(ui.Err, "Updated log.batch.db_path to %s\n", logDbPath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	_, err = fmt.Fprintln(ui.Err, "Log database initialized successfully.")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteOutput, err)
	}

	return nil
}

func updateLogPathInConfig(secureStore config.SecureStore, logPath string) error {
	var cfg config.Config
	decryptedBytes, _, err := secureStore.Get(config.ScopeApplication, 0)
	if err == nil && len(decryptedBytes) > 0 {
		err = toml.Unmarshal(decryptedBytes, &cfg)
		if err != nil {
			return fmt.Errorf("%w: failed to parse config for log path update: %w", ErrUpdateLogPath, err)
		}
	}

	cfg.Log.Batch.DbPath = logPath

	tomlBytes, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal config for log path update: %w", ErrConfigMarshal, err)
	}

	err = secureStore.Save(config.ScopeApplication, tomlBytes, "toml", fmt.Sprintf("Updated log.batch.db_path to %s", logPath))
	if err != nil {
		return fmt.Errorf("%w: failed to save log path update: %w", ErrSecureStoreSave, err)
	}

	return nil
}
