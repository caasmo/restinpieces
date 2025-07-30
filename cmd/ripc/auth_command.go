package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
)

var (
	ErrUnknownAuthSubcommand = errors.New("unknown auth subcommand")
)

func printAuthUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s auth <subcommand> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Manages authentication settings.\n\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  rotate-jwt-secrets    Rotate all JWT secrets\n")
	fmt.Fprintf(os.Stderr, "  add-oauth2 <provider>   Add a new OAuth2 provider\n")
	fmt.Fprintf(os.Stderr, "  rm-oauth2 <provider>    Remove an OAuth2 provider\n")
}

func handleAuthCommand(secureStore config.SecureStore, commandArgs []string) {
	if len(commandArgs) < 1 {
		printAuthUsage()
		os.Exit(1)
	}

	subcommand, subcommandArgs, err := parseAuthSubcommand(commandArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printAuthUsage()
		os.Exit(1)
	}

	switch subcommand {
	case "rotate-jwt-secrets":
		handleRotateJwtSecretsCommand(secureStore)
	case "add-oauth2":
		handleOAuth2Command(secureStore, subcommandArgs[0])
	case "rm-oauth2":
		handleRmOAuth2Command(secureStore, subcommandArgs[0])
	default:
		// This case should ideally not be reached if parseAuthSubcommand is correct
		fmt.Fprintf(os.Stderr, "Error: unknown auth subcommand: %s\n", subcommand)
		printAuthUsage()
		os.Exit(1)
	}
}

func parseAuthSubcommand(commandArgs []string) (string, []string, error) {
	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "rotate-jwt-secrets":
		if len(subcommandArgs) > 0 {
			return "", nil, fmt.Errorf("'rotate-jwt-secrets' does not take any arguments: %w", ErrTooManyArguments)
		}
		return subcommand, nil, nil
	case "add-oauth2":
		if len(subcommandArgs) != 1 {
			return "", nil, fmt.Errorf("'add-oauth2' requires exactly one provider name argument: %w", ErrMissingArgument)
		}
		return subcommand, subcommandArgs, nil
	case "rm-oauth2":
		if len(subcommandArgs) != 1 {
			return "", nil, fmt.Errorf("'rm-oauth2' requires exactly one provider name argument: %w", ErrMissingArgument)
		}
		return subcommand, subcommandArgs, nil
	default:
		return "", nil, fmt.Errorf("'%s': %w", subcommand, ErrUnknownAuthSubcommand)
	}
}