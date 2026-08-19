package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

func printDumpUsage(w io.Writer) {
	help := Spec{
		Usage:       "dump [options]",
		Description: "Dumps the configuration.",
		Options: []OptSpec{
			commandOptions.Opt("scope"),
			commandOptions.Opt("zero"),
			commandOptions.Opt("runtime"),
		},
		Examples: []string{
			"ripc dump",
			"ripc dump --zero",
			"ripc dump --runtime",
			"ripc dump --scope my-app",
		},
	}
	help.Print(w, prog)
}

// handleDumpCommand parses the arguments for the 'dump' command and executes
// the core logic, returning any error to the caller.
func handleDumpCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parseDumpArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printDumpUsage(ui.Out)
			return nil
		}
		printDumpUsage(ui.Err)
		return err
	}
	return dumpConfig(ui, secureStore, opts.Scope, opts.Zero, opts.Runtime)
}

// dumpConfig contains the testable core logic for dumping configuration.
// It accepts UI for output, making it easy to test.
func dumpConfig(ui UI, secureStore config.SecureStore, scope string, zero bool, runtime bool) error {
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
		_, err = ui.Out.Write(out)
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
		_, err = ui.Out.Write(out)
		if err != nil {
			return fmt.Errorf("%w: failed to write runtime config to stdout: %w", ErrWriteOutput, err)
		}
		return nil
	}

	// Raw: write decrypted bytes directly
	_, err = ui.Out.Write(decryptedData)
	if err != nil {
		return fmt.Errorf("%w: failed to write raw config to stdout: %w", ErrWriteOutput, err)
	}
	return nil
}

// DumpOptions holds the parsed options for the 'dump' command.
type DumpOptions struct {
	Scope   string // --scope
	Zero    bool   // --zero
	Runtime bool   // --runtime
}

// parseDumpArgs parses the arguments for the 'dump' command.
func parseDumpArgs(args []string) (DumpOptions, error) {
	dumpCmd := flag.NewFlagSet("dump", flag.ContinueOnError)
	dumpCmd.SetOutput(io.Discard)
	scopeOpt := commandOptions.Opt("scope")
	zeroOpt := commandOptions.Opt("zero")
	runtimeOpt := commandOptions.Opt("runtime")

	var opts DumpOptions
	dumpCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	dumpCmd.BoolVar(&opts.Zero, "zero", false, zeroOpt.Usage)
	dumpCmd.BoolVar(&opts.Runtime, "runtime", false, runtimeOpt.Usage)

	err := dumpCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return DumpOptions{}, flag.ErrHelp
		}
		return DumpOptions{}, fmt.Errorf("parsing dump flags: %w: %v", ErrInvalidFlag, err)
	}
	if dumpCmd.NArg() > 0 {
		return DumpOptions{}, fmt.Errorf("'dump' command does not take any arguments: %w", ErrTooManyArguments)
	}
	if opts.Zero && opts.Runtime {
		return DumpOptions{}, fmt.Errorf("--zero and --runtime are mutually exclusive: %w", ErrInvalidFlag)
	}
	return opts, nil
}
