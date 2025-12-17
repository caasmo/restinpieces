package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

func TestCommandHelp_Print(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("option", "default", "a test option")

	help := CommandHelp{
		Usage:       "test-usage",
		Description: "test description",
		Subcommands: []SubcommandGroup{
			{
				Title: "Test Group",
				Subcommands: []Subcommand{
					{"sub", "sub description"},
				},
			},
		},
		Options:       fs,
		GlobalOptions: fs,
		Examples: []string{
			"example 1",
		},
	}

	var buf bytes.Buffer
	help.Print(&buf, "test-parent")

	output := buf.String()

	expectedSubstrings := []string{
		"Usage:",
		"test-usage",
		"Description:",
		"test description",
		"Subcommands:",
		"Test Group",
		"sub",
		"sub description",
		"Options:",
		"-option",
		"Global Options:",
		"Examples:",
		"example 1",
		"For detailed help on a subcommand:",
		"test-parent <subcommand> --help",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(output, sub) {
			t.Errorf("expected output to contain %q, but it did not.\n\nGot:\n%s", sub, output)
		}
	}
}
