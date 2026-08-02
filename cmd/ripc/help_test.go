package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSpec_Print(t *testing.T) {
	spec := Spec{
		Usage:       "<subcommand> [options]",
		Description: "test description",
		Args: []ArgSpec{
			{"filter", "optional filter"},
		},
		Subcommands: []SubcommandGroup{
			{
				Title: "Test Group",
				Subcommands: []Subcommand{
					{"sub", "sub description"},
				},
			},
		},
		Options: []OptSpec{
			{Name: "option", Meta: "string", DefaultValue: "default", Usage: "a test option"},
		},
		GlobalOptions: []OptSpec{
			{Name: "global", Meta: "string", Usage: "a global option"},
		},
		Examples: []string{
			"example 1",
		},
	}

	var buf bytes.Buffer
	spec.Print(&buf, "test", "parent")

	output := buf.String()

	expectedSubstrings := []string{
		"Usage:",
		"test <subcommand> [options]",
		"Description:",
		"test description",
		"Arguments:",
		"filter",
		"optional filter",
		"Subcommands:",
		"Test Group",
		"sub",
		"sub description",
		"Options:",
		"-option string",
		"a test option (default: \"default\")",
		"Global Options:",
		"-global string",
		"Examples:",
		"example 1",
		"For detailed help on a subcommand:",
		"test parent <subcommand> --help",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(output, sub) {
			t.Errorf("expected output to contain %q, but it did not.\n\nGot:\n%s", sub, output)
		}
	}
}
