package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

// handleDumpCommand is the command-level wrapper. It executes the core logic
// and handles exiting the process on error.
func handleDumpCommand(secureStore config.SecureStore, scope string, zero bool, runtime bool, ui UI) {
	if err := dumpConfig(ui.Out, secureStore, scope, zero, runtime); err != nil {
		fprintErr(ui.Err, err)
		os.Exit(1)
	}
}

// dumpConfig contains the testable core logic for dumping configuration.
// It accepts io.Writer for output, making it easy to test.
func dumpConfig(stdout io.Writer, secureStore config.SecureStore, scope string, zero bool, runtime bool) error {
	if scope == "" {
		scope = config.ScopeApplication
	}
	decryptedData, _, err := secureStore.Get(scope, 0) // generation 0 = latest
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve latest config for scope '%s': %w", ErrSecureStoreGet, scope, err)
	}

	if zero {
		normalized := &config.Config{}
		err = toml.Unmarshal(decryptedData, normalized)
		if err != nil {
			return fmt.Errorf("failed to parse config for zero-default dump: %w", err)
		}
		out, err := toml.Marshal(normalized)
		if err != nil {
			return fmt.Errorf("failed to serialize zero-default config: %w", err)
		}
		_, err = stdout.Write(out)
		if err != nil {
			return fmt.Errorf("%w: failed to write zero-default config to stdout: %w", ErrWriteOutput, err)
		}
		return nil
	}

	if runtime {
		merged := config.NewDefaultConfig()
		if len(decryptedData) > 0 {
			err = toml.Unmarshal(decryptedData, merged)
			if err != nil {
				return fmt.Errorf("failed to parse stored config for runtime dump: %w", err)
			}
		}
		out, err := toml.Marshal(merged)
		if err != nil {
			return fmt.Errorf("failed to serialize runtime config: %w", err)
		}
		_, err = stdout.Write(out)
		if err != nil {
			return fmt.Errorf("%w: failed to write runtime config to stdout: %w", ErrWriteOutput, err)
		}
		return nil
	}

	// Raw: write decrypted bytes directly
	_, err = stdout.Write(decryptedData)
	if err != nil {
		return fmt.Errorf("%w: failed to write raw config to stdout: %w", ErrWriteOutput, err)
	}
	return nil
}

// parseDumpArgs parses the arguments for the 'dump' subcommand.
func parseDumpArgs(args []string) (scope string, zero bool, runtime bool, err error) {
	dumpCmd := flag.NewFlagSet("dump", flag.ContinueOnError)
	dumpCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Options["scope"]
	zeroOpt := commandConfig.Options["zero"]
	runtimeOpt := commandConfig.Options["runtime"]
	dumpScope := dumpCmd.String("scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	dumpZero := dumpCmd.Bool("zero", false, zeroOpt.Usage)
	dumpRuntime := dumpCmd.Bool("runtime", false, runtimeOpt.Usage)

	if err := dumpCmd.Parse(args); err != nil {
		return "", false, false, fmt.Errorf("parsing dump flags: %w: %v", ErrInvalidFlag, err)
	}
	if dumpCmd.NArg() > 0 {
		return "", false, false, fmt.Errorf("'dump' command does not take any arguments: %w", ErrTooManyArguments)
	}
	if *dumpZero && *dumpRuntime {
		return "", false, false, fmt.Errorf("--zero and --runtime are mutually exclusive: %w", ErrInvalidFlag)
	}
	return *dumpScope, *dumpZero, *dumpRuntime, nil
}
