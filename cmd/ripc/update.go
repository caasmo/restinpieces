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

// updater produces a fresh value for one configuration path by writing it directly into the tree, then reports what it wrote.
type updater interface {
	Update(tree *toml.Tree, path string) error
	Print(ui UI, tree *toml.Tree, path string) error
}

// secretUpdater produces a fresh random secret.
type secretUpdater struct{}

func (secretUpdater) Update(tree *toml.Tree, path string) error {
	tree.Set(path, crypto.RandomString(32, crypto.AlphanumericAlphabet))
	return nil
}

// Print reports the fresh secret by prefix and length only. Full secrets never reach scrollback.
func (secretUpdater) Print(ui UI, tree *toml.Tree, path string) error {
	value, _ := tree.Get(path).(string)
	prefix := value
	if len(prefix) > 7 {
		prefix = prefix[:7]
	}
	_, err := fmt.Fprintf(ui.Err, "%s = %s... (%d chars)\n", path, prefix, len(value))
	if err != nil {
		return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, err)
	}

	return nil
}

// userAgentUpdater produces the block_user_agent.agents slice from the upstream user-agent list.
type userAgentUpdater struct{}

func (userAgentUpdater) Update(tree *toml.Tree, path string) error {
	agents, err := fetchUserAgents(userAgentURL)
	if err != nil {
		return err
	}

	tree.Set(path, agents)
	return nil
}

// Print reports the filled agents value.
func (userAgentUpdater) Print(ui UI, tree *toml.Tree, path string) error {
	_, err := fmt.Fprintf(ui.Err, "%s = %v\n", path, tree.Get(path))
	if err != nil {
		return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, err)
	}

	return nil
}

// updaters maps each configuration path to the updater that produces its value. TLS is one entry: the pair moves together, so a filter can never match half of it.
var updaters = map[string]updater{
	"jwt.auth_secret":                   secretUpdater{},
	"jwt.password_reset_secret":         secretUpdater{},
	"jwt.email_change_otp_secret":       secretUpdater{},
	"jwt.verification_email_otp_secret": secretUpdater{},
	"jwt.oauth2_state_secret":           secretUpdater{},
	"block_user_agent.agents":           userAgentUpdater{},
	"server.tls":                        tlsUpdater{},
}

func printUpdateUsage(w io.Writer) {
	help := Spec{
		Usage:       "update <filter>",
		Description: "Fills configuration values in place.",
		Args: []ArgSpec{
			{"filter", "Required substring filter on updatable paths"},
		},
		Options: []OptSpec{
			commandOptions.Opt("scope"),
			commandOptions.Opt("desc"),
		},
		Examples: []string{
			"ripc update jwt.auth_secret",
			"ripc update jwt",
			"ripc update tls",
		},
	}
	help.Print(w, prog)
}

// UpdateOptions holds the parsed options for the 'update' command.
type UpdateOptions struct {
	Scope  string // --scope
	Desc   string // --desc
	Filter string // optional positional filter argument
}

// handleUpdateCommand parses the arguments for the 'update' command and executes the core logic, returning any error to the caller.
func handleUpdateCommand(secureStore config.SecureStore, args []string, ui UI) error {
	opts, err := parseUpdateArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUpdateUsage(ui.Out)
			return nil
		}
		printUpdateUsage(ui.Err)
		return err
	}
	return updateValues(ui, secureStore, opts.Scope, opts.Desc, opts.Filter)
}

// parseUpdateArgs parses the arguments for the 'update' command.
func parseUpdateArgs(args []string) (UpdateOptions, error) {
	updateCmd := flag.NewFlagSet("update", flag.ContinueOnError)
	updateCmd.SetOutput(io.Discard)
	scopeOpt := commandOptions.Opt("scope")
	descOpt := commandOptions.Opt("desc")

	var opts UpdateOptions
	updateCmd.StringVar(&opts.Scope, "scope", scopeOpt.DefaultValue, scopeOpt.Usage)
	updateCmd.StringVar(&opts.Desc, "desc", descOpt.DefaultValue, descOpt.Usage)

	err := updateCmd.Parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return UpdateOptions{}, flag.ErrHelp
		}
		return UpdateOptions{}, fmt.Errorf("parsing update flags: %w: %v", ErrInvalidFlag, err)
	}
	if updateCmd.NArg() != 1 {
		return UpdateOptions{}, fmt.Errorf("'update' requires exactly one filter argument: %w", ErrMissingArgument)
	}
	opts.Filter = updateCmd.Arg(0)
	return opts, nil
}

// updateValues contains the testable core logic for filling values in place. It accepts UI for output, making it easy to test.
func updateValues(ui UI, secureCfg config.SecureStore, scope string, description string, filter string) error {
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

	matched := make([]string, 0, len(updaters))
	for path := range updaters {
		if strings.Contains(path, filter) {
			matched = append(matched, path)
		}
	}
	sort.Strings(matched)

	if len(matched) == 0 {
		_, writeErr := fmt.Fprintf(ui.Err, "No updatable paths matching '%s' found in scope '%s'.\n", filter, scope)
		if writeErr != nil {
			return fmt.Errorf("%w: failed to write output: %w", ErrWriteOutput, writeErr)
		}
		return nil
	}

	for _, path := range matched {
		if updateErr := updaters[path].Update(tree, path); updateErr != nil {
			return updateErr
		}
	}

	updatedTomlBytes, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal updated config: %w", ErrConfigMarshal, err)
	}

	if description == "" {
		description = fmt.Sprintf("Updated '%s'", strings.Join(matched, ", "))
	}

	err = secureCfg.Save(scope, updatedTomlBytes, fileFormat, description)
	if err != nil {
		return fmt.Errorf("%w: failed to save updated config for scope '%s': %w", ErrSecureStoreSave, scope, err)
	}

	for _, path := range matched {
		if printErr := updaters[path].Print(ui, tree, path); printErr != nil {
			return printErr
		}
	}
	return nil
}
