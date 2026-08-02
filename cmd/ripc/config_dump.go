package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/caasmo/restinpieces/config"
	"github.com/pelletier/go-toml/v2"
)

func printConfigDumpUsage(w io.Writer) {
	help := Spec{
		Usage:       "config dump [options]",
		Description: "Dumps the configuration.",
		Options: []OptSpec{
			commandConfig.Opt("scope"),
			commandConfig.Opt("zero"),
			commandConfig.Opt("runtime"),
		},
		Examples: []string{
			"ripc config dump",
			"ripc config dump --zero",
			"ripc config dump --runtime",
			"ripc config dump --scope my-app",
		},
	}
	help.Print(w, prog, "config", "dump")
}

// handleConfigDumpCommand is the command-level wrapper. It executes the core logic
// and returns any error to the caller.
func handleConfigDumpCommand(secureStore config.SecureStore, opts ConfigDumpOptions, ui UI) error {
	return dumpConfig(ui.Out, secureStore, opts.Scope, opts.Zero, opts.Runtime)
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

// ConfigDumpOptions holds the parsed options for the 'config dump' subcommand.
type ConfigDumpOptions struct {
	Scope   string // --scope
	Zero    bool   // --zero
	Runtime bool   // --runtime
}

// parseConfigDumpArgs parses the arguments for the 'dump' subcommand.
func parseConfigDumpArgs(args []string) (ConfigDumpOptions, error) {
	dumpCmd := flag.NewFlagSet("dump", flag.ContinueOnError)
	dumpCmd.SetOutput(io.Discard)
	scopeOpt := commandConfig.Opt("scope")
	zeroOpt := commandConfig.Opt("zero")
	runtimeOpt := commandConfig.Opt("runtime")

	var opts ConfigDumpOptions
	dumpCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	dumpCmd.BoolVar(&opts.Zero, "zero", false, zeroOpt.Usage)
	dumpCmd.BoolVar(&opts.Runtime, "runtime", false, runtimeOpt.Usage)

	err := dumpCmd.Parse(args)
	if err != nil {
		return ConfigDumpOptions{}, fmt.Errorf("parsing dump flags: %w: %v", ErrInvalidFlag, err)
	}
	if dumpCmd.NArg() > 0 {
		return ConfigDumpOptions{}, fmt.Errorf("'dump' command does not take any arguments: %w", ErrTooManyArguments)
	}
	if opts.Zero && opts.Runtime {
		return ConfigDumpOptions{}, fmt.Errorf("--zero and --runtime are mutually exclusive: %w", ErrInvalidFlag)
	}
	return opts, nil
}
