package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/caasmo/restinpieces/config"
)

func handleAuthCommand(secureStore config.SecureStore, commandArgs []string) {
	authCmd := flag.NewFlagSet("auth", flag.ExitOnError)
	authCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s auth <subcommand> [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Manages authentication settings.\n\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  rotate-jwt-secrets    Rotate all JWT secrets\n")
		fmt.Fprintf(os.Stderr, "  add-oauth2 <provider>   Add a new OAuth2 provider\n")
		fmt.Fprintf(os.Stderr, "  rm-oauth2 <provider>    Remove an OAuth2 provider\n")
	}

	if len(commandArgs) < 1 {
		authCmd.Usage()
		os.Exit(1)
	}

	subcommand := commandArgs[0]
	subcommandArgs := commandArgs[1:]

	switch subcommand {
	case "rotate-jwt-secrets":
		if len(subcommandArgs) > 0 {
			fmt.Fprintf(os.Stderr, "Error: 'rotate-jwt-secrets' does not take any arguments\n")
			authCmd.Usage()
			os.Exit(1)
		}
		handleRotateJwtSecretsCommand(secureStore)
	case "add-oauth2":
		if len(subcommandArgs) < 1 {
			fmt.Fprintf(os.Stderr, "Error: 'add-oauth2' requires provider name argument\n")
			authCmd.Usage()
			os.Exit(1)
		}
		handleOAuth2Command(secureStore, subcommandArgs[0])
	case "rm-oauth2":
		if len(subcommandArgs) < 1 {
			fmt.Fprintf(os.Stderr, "Error: 'rm-oauth2' requires provider name argument\n")
			authCmd.Usage()
			os.Exit(1)
		}
		handleRmOAuth2Command(secureStore, subcommandArgs[0])
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown auth subcommand: %s\n", subcommand)
		authCmd.Usage()
		os.Exit(1)
	}
}