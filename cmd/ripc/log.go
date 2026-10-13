package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

// logPollInterval is the pause between two polls once the reader has caught
// up with the writer.
const logPollInterval = 1 * time.Second

// logChunkSize is how many log ids one backward query covers while the
// command collects the newest records.
const logChunkSize = 1000

// logDefaultNumber is how many records the command prints when -n is not
// given.
const logDefaultNumber = 20

// logSpec describes the log command. It is the single source of truth for
// the help text and for flag registration.
var logSpec = Spec{
	Usage:       "log [options] [filter]",
	Description: "Reads log records from the log database.",
	Args: []ArgSpec{
		{"filter", "Optional substring filter on the log record data"},
	},
	Options: []OptSpec{
		{Name: "f", Usage: "Keep following new log records"},
		{Name: "n", Meta: "N", DefaultValue: strconv.Itoa(logDefaultNumber), Usage: "Number of log records to print first"},
	},
	Examples: []string{
		"ripc log",
		"ripc log 404",
		"ripc log -n 50",
		"ripc log -f",
	},
}

func printLogUsage(w io.Writer) {
	logSpec.Print(w, prog)
}

// handleLogCommand parses the arguments for the 'log' command and executes
// the core logic, returning any error to the caller.
func handleLogCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parseLogArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printLogUsage(ui.Out)
			return nil
		}
		printLogUsage(ui.Err)
		return err
	}
	return logTail(ui, secureStore, opts)
}

// LogOptions holds the parsed options for the 'log' command.
type LogOptions struct {
	Follow bool   // -f
	Number int    // -n
	Filter string // optional positional filter argument
}

// parseLogArgs parses the arguments for the 'log' command.
func parseLogArgs(args []string) (LogOptions, error) {
	logCmd := flag.NewFlagSet("log", flag.ContinueOnError)
	logCmd.SetOutput(io.Discard)
	followOpt := logSpec.Opt("f")
	numberOpt := logSpec.Opt("n")

	var opts LogOptions
	logCmd.BoolVar(&opts.Follow, "f", false, followOpt.Usage)
	logCmd.IntVar(&opts.Number, "n", logDefaultNumber, numberOpt.Usage)

	err := logCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return LogOptions{}, flag.ErrHelp
		}
		return LogOptions{}, fmt.Errorf("parsing log flags: %w: %v", ErrInvalidFlag, err)
	}
	if logCmd.NArg() > 1 {
		return LogOptions{}, fmt.Errorf("'log' command takes at most one filter argument: %w", ErrTooManyArguments)
	}
	if logCmd.NArg() > 0 {
		opts.Filter = logCmd.Arg(0)
	}
	if opts.Number < 0 {
		return LogOptions{}, fmt.Errorf("'log' command does not accept a negative -n value: %w", ErrInvalidFlag)
	}
	return opts, nil
}

// logTail prints the newest records that match the filter, oldest first.
// With Follow it keeps printing new matching records as they arrive and runs
// until the process is interrupted.
func logTail(ui UI, secureStore config.SecureStore, opts LogOptions) (err error) {
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

	maxLogID, err := ldb.maxID()
	if err != nil {
		return err
	}

	// Collect the newest matching records by walking the id range backwards
	// in chunks. Records come back newest first.
	cursor := maxLogID + 1
	records := []logRecord{}
	for len(records) < opts.Number && cursor > 0 {
		chunk, readErr := ldb.tailBackwards(cursor, logChunkSize, opts.Filter)
		if readErr != nil {
			return readErr
		}
		records = append(records, chunk...)
		cursor -= logChunkSize
	}

	if len(records) > opts.Number {
		records = records[:opts.Number]
	}
	// Chunks come back newest first, so flip the collection before printing.
	slices.Reverse(records)

	writeErr := printRecords(ui.Out, records)
	if writeErr != nil {
		return writeErr
	}

	if !opts.Follow {
		return nil
	}

	// Follow new records from the snapshot taken before the backfill. Sleep
	// only when a poll finds nothing, so a backlog drains without pauses.
	cursor = maxLogID
	for {
		newRecords, pollErr := ldb.tail(cursor, opts.Filter)
		if pollErr != nil {
			return pollErr
		}

		if len(newRecords) == 0 {
			time.Sleep(logPollInterval)
			continue
		}

		writeErr = printRecords(ui.Out, newRecords)
		if writeErr != nil {
			return writeErr
		}
		cursor = newRecords[len(newRecords)-1].ID
	}
}

// printRecords writes one line per record, oldest first.
func printRecords(w io.Writer, records []logRecord) error {
	for _, record := range records {
		_, err := fmt.Fprintf(w, "%s %s %s %s\n", record.Created, slog.Level(record.Level).String(), record.Message, record.Data)
		if err != nil {
			return fmt.Errorf("%w: failed to write log record: %w", ErrWriteOutput, err)
		}
	}
	return nil
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
