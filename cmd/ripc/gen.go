package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	toml "github.com/pelletier/go-toml"
)

// generator produces a fresh value for one configuration path.
type generator interface {
	Generate() (string, error)
}

// secretGenerator produces a fresh random secret.
type secretGenerator struct{}

// Generate returns a fresh 32-character alphanumeric secret.
func (secretGenerator) Generate() (string, error) {
	return crypto.RandomString(32, crypto.AlphanumericAlphabet), nil
}

// userAgentRegexpGenerator produces the block_ua_list.list regular expression
// from the upstream user-agent list.
type userAgentRegexpGenerator struct{}

func (userAgentRegexpGenerator) Generate() (string, error) {
	agents, err := fetchUserAgents(userAgentURL)
	if err != nil {
		return "", err
	}

	return config.BuildUserAgentRegexp(agents)
}

// genFuncs maps each configuration path to the generator that produces its
// value.
var genFuncs = map[string]generator{
	"jwt.auth_secret":                   secretGenerator{},
	"jwt.password_reset_secret":         secretGenerator{},
	"jwt.email_change_otp_secret":       secretGenerator{},
	"jwt.verification_email_otp_secret": secretGenerator{},
	"jwt.oauth2_state_secret":           secretGenerator{},
	"block_ua_list.list":                userAgentRegexpGenerator{},
}

func printGenUsage(w io.Writer) {
	help := Spec{
		Usage:       "gen [options] [filter]",
		Description: "Generates fresh values for configuration values.",
		Args: []ArgSpec{
			{"filter", "Optional substring filter on generatable paths"},
		},
		Options: []OptSpec{
			commandOptions.Opt("scope"),
			commandOptions.Opt("desc"),
		},
		Examples: []string{
			"ripc gen jwt.auth_secret",
			"ripc gen jwt",
			"ripc gen block_ua_list",
		},
	}
	help.Print(w, prog)
}

// GenOptions holds the parsed options for the 'gen' command.
type GenOptions struct {
	Scope  string // --scope
	Desc   string // --desc
	Filter string // optional positional filter argument
}

// handleGenCommand parses the arguments for the 'gen' command and executes the core logic, returning any error to the caller.
func handleGenCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parseGenArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printGenUsage(ui.Out)
			return nil
		}
		printGenUsage(ui.Err)
		return err
	}
	return generate(ui, secureStore, opts.Scope, opts.Desc, opts.Filter)
}

// parseGenArgs parses the arguments for the 'gen' command.
func parseGenArgs(args []string) (GenOptions, error) {
	genCmd := flag.NewFlagSet("gen", flag.ContinueOnError)
	genCmd.SetOutput(io.Discard)
	scopeOpt := commandOptions.Opt("scope")
	descOpt := commandOptions.Opt("desc")

	var opts GenOptions
	genCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	genCmd.StringVar(&opts.Desc, "desc", descOpt.DefaultValue, descOpt.Usage)

	err := genCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return GenOptions{}, flag.ErrHelp
		}
		return GenOptions{}, fmt.Errorf("parsing gen flags: %w: %v", ErrInvalidFlag, err)
	}
	if genCmd.NArg() > 1 {
		return GenOptions{}, fmt.Errorf("'gen' command takes at most one filter argument: %w", ErrTooManyArguments)
	}
	if genCmd.NArg() > 0 {
		opts.Filter = genCmd.Arg(0)
	}
	return opts, nil
}

// generate contains the testable core logic for generating fresh values. It accepts UI for output, making it easy to test.
func generate(ui UI, secureCfg config.SecureStore, scope string, description string, filter string) error {
	if scope == "" {
		scope = config.ScopeApplication
	}

	decryptedData, fileFormat, err := secureCfg.Get(scope, 0) // generation 0 = latest
	if err != nil {
		return fmt.Errorf("%w: failed to retrieve latest config for scope '%s': %w", ErrSecureStoreGet, scope, err)
	}

	tree, err := toml.LoadBytes(decryptedData)
	if err != nil {
		return fmt.Errorf("%w: failed to load config data for scope '%s': %w", ErrConfigUnmarshal, scope, err)
	}

	matched := make([]string, 0, len(genFuncs))
	for path := range genFuncs {
		if filter == "" || strings.Contains(path, filter) {
			matched = append(matched, path)
		}
	}
	sort.Strings(matched)

	if len(matched) == 0 {
		_, writeErr := fmt.Fprintf(ui.Err, "No generatable paths matching '%s' found in scope '%s'.\n", filter, scope)
		if writeErr != nil {
			return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, writeErr)
		}
		return nil
	}

	for _, path := range matched {
		if !tree.Has(path) {
			return fmt.Errorf("%w: path '%s' not found in config for scope '%s'", ErrPathNotFound, path, scope)
		}
	}

	for _, path := range matched {
		fresh, genErr := genFuncs[path].Generate()
		if genErr != nil {
			return genErr
		}
		tree.Set(path, fresh)
	}

	updatedTomlBytes, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal updated config: %w", ErrConfigMarshal, err)
	}

	if description == "" {
		description = fmt.Sprintf("Generated '%s'", strings.Join(matched, ", "))
	}

	err = secureCfg.Save(scope, updatedTomlBytes, fileFormat, description)
	if err != nil {
		return fmt.Errorf("%w: failed to save updated config for scope '%s': %w", ErrSecureStoreSave, scope, err)
	}

	for _, path := range matched {
		_, writeErr := fmt.Fprintf(ui.Err, "%s = %v\n", path, tree.Get(path))
		if writeErr != nil {
			return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, writeErr)
		}
	}
	return nil
}
