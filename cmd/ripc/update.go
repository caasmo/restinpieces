package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/caasmo/restinpieces/config"
	toml "github.com/pelletier/go-toml"
)

// ErrUnknownUpdateLabel reports an update argument that names no known label or registered configuration path.
var ErrUnknownUpdateLabel = errors.New("unknown update label")

// TOML paths used by the update command, matching the toml tags in the config
// structs: Jwt, BlockUserAgent and Server.Tls (config/config.go) and Acme
// (config/acme.go).
const (
	tomlPathJwtAuthSecret                 = "jwt.auth_secret"
	tomlPathJwtPasswordResetSecret        = "jwt.password_reset_secret"
	tomlPathJwtEmailChangeOtpSecret       = "jwt.email_change_otp_secret"
	tomlPathJwtVerificationEmailOtpSecret = "jwt.verification_email_otp_secret"
	tomlPathJwtOauth2StateSecret          = "jwt.oauth2_state_secret"
	tomlPathBlockUserAgentAgents          = "block_user_agent.agents"
	tomlPathServerTLS                     = "server.tls"
	tomlPathServerTLSCertificate          = "server.tls.certificate"
	tomlPathServerTLSPrivateKey           = "server.tls.private_key"
	tomlPathAcmeCertificate               = "acme.certificate"
	tomlPathAcmePrivateKey                = "acme.private_key"
)

// updater produces a fresh value for one update argument by writing it directly into the tree, then reports what it wrote. The argument is a label or a single configuration path.
type updater interface {
	Update(tree *toml.Tree, arg string) error
	Print(ui UI, tree *toml.Tree, arg string) error
}

// updateGroup is one update group: the configuration paths it covers and the
// updater that writes fresh values into them. A group with no paths, such as
// block_user_agent or tls, is triggered by its own name only.
type updateGroup struct {
	paths   []string
	updater updater
}

// updateGroups maps each user-facing label to its paths and updater.
var updateGroups = map[string]updateGroup{
	"jwt": {
		paths: []string{
			tomlPathJwtAuthSecret,
			tomlPathJwtPasswordResetSecret,
			tomlPathJwtEmailChangeOtpSecret,
			tomlPathJwtVerificationEmailOtpSecret,
			tomlPathJwtOauth2StateSecret,
		},
		updater: jwtUpdater{},
	},
	"block_user_agent": {
		updater: userAgentUpdater{},
	},
	"tls": {
		updater: tlsUpdater{},
	},
}

func printUpdateUsage(w io.Writer) {
	help := Spec{
		Usage:       "update <label>",
		Description: "Generates new secrets, refreshes the bot block list, or activates the staged TLS certificate.",
		Args: []ArgSpec{
			{"label", "Label (jwt, tls, block_user_agent) or configuration path"},
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
	Scope string // --scope
	Desc  string // --desc
	Label string // required positional label argument
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
	return updateValues(ui, secureStore, opts.Scope, opts.Desc, opts.Label)
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
		return UpdateOptions{}, fmt.Errorf("'update' requires exactly one label argument: %w", ErrMissingArgument)
	}
	opts.Label = updateCmd.Arg(0)
	return opts, nil
}

// updateValues contains the testable core logic for writing fresh values. It accepts UI for output, making it easy to test.
func updateValues(ui UI, secureCfg config.SecureStore, scope string, description string, arg string) error {
	group, ok := resolveUpdateGroup(arg)
	if !ok {
		return fmt.Errorf("%w: %q; use a label (%s) or a configuration path", ErrUnknownUpdateLabel, arg, strings.Join(knownUpdateLabels(), ", "))
	}

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

	err = group.updater.Update(tree, arg)
	if err != nil {
		return err
	}

	updatedTomlBytes, err := toml.Marshal(tree)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal updated config: %w", ErrConfigMarshal, err)
	}

	if description == "" {
		description = fmt.Sprintf("Updated '%s'", strings.Join(tomlPaths(arg), ", "))
	}

	err = secureCfg.Save(scope, updatedTomlBytes, fileFormat, description)
	if err != nil {
		return fmt.Errorf("%w: failed to save updated config for scope '%s': %w", ErrSecureStoreSave, scope, err)
	}

	err = group.updater.Print(ui, tree, arg)
	if err != nil {
		return err
	}
	return nil
}

// resolveUpdateGroup returns the update group the argument names: the group
// whose label is the argument, or the group that covers the argument as one of
// its configuration paths.
func resolveUpdateGroup(arg string) (updateGroup, bool) {
	for label, group := range updateGroups {
		if label == arg || slices.Contains(group.paths, arg) {
			return group, true
		}
	}

	return updateGroup{}, false
}

// tomlPaths returns the configuration paths the argument names: the paths of a
// group, or the argument itself when it is a configuration path.
func tomlPaths(arg string) []string {
	group, ok := updateGroups[arg]
	if !ok {
		return []string{arg}
	}

	paths := make([]string, len(group.paths))
	copy(paths, group.paths)
	sort.Strings(paths)
	return paths
}

// knownUpdateLabels returns the registered label names in stable order.
func knownUpdateLabels() []string {
	labels := make([]string, 0, len(updateGroups))
	for label := range updateGroups {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}
