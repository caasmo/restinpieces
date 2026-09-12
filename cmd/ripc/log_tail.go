package main

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

// logPollInterval is the pause between two log polls once the tail has
// caught up with the writer.
const logPollInterval = 3 * time.Second

// handleLogTailCommand is the command-level wrapper. It executes the core
// logic and returns any error to the caller.
func handleLogTailCommand(secureStore config.SecureStore, ui UI) error {
	return logTail(ui, secureStore)
}

// logTail follows the log database, printing every record appended after the
// command started. It polls every logPollInterval and runs until the process
// is interrupted.
func logTail(ui UI, secureStore config.SecureStore) (err error) {
	logDbPath, err := readLogDbPath(secureStore)
	if err != nil {
		return err
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

	lastID, err := ldb.maxID()
	if err != nil {
		return err
	}

	for {
		records, pollErr := ldb.tail(lastID)
		if pollErr != nil {
			return pollErr
		}

		for _, record := range records {
			_, writeErr := fmt.Fprintf(ui.Out, "%s %s %s %s\n", record.Created, slog.Level(record.Level).String(), record.Message, record.Data)
			if writeErr != nil {
				return fmt.Errorf("%w: failed to write log record: %w", ErrWriteOutput, writeErr)
			}
			lastID = record.ID
		}

		time.Sleep(logPollInterval)
	}
}

// readLogDbPath resolves log.batch.db_path from the application config,
// merging framework defaults like the running application does.
func readLogDbPath(secureStore config.SecureStore) (string, error) {
	decryptedData, _, err := secureStore.Get(config.ScopeApplication, 0)
	if err != nil {
		return "", fmt.Errorf("%w: failed to retrieve config for log tail: %w", ErrSecureStoreGet, err)
	}

	cfg := config.NewDefaultConfig()
	if len(decryptedData) > 0 {
		err = toml.Unmarshal(decryptedData, cfg)
		if err != nil {
			return "", fmt.Errorf("%w: failed to parse config for log tail: %w", ErrConfigUnmarshal, err)
		}
	}

	if cfg.Log.Batch.DbPath == "" {
		return "", fmt.Errorf("log database path is not configured: run 'ripc log init <logpath>' first")
	}

	return cfg.Log.Batch.DbPath, nil
}
