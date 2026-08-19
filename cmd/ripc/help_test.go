package main

import (
	"bytes"
	"io"
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

// TestPrintUsageFuncs covers every group and leaf usage printer. The printers
// are pure renderers (a Spec literal + one Print call), so a table test
// asserting key sections is the whole of their behavior.
func TestPrintUsageFuncs(t *testing.T) {
	testCases := []struct {
		name    string
		usage   func(w io.Writer)
		want    []string
		notWant []string
	}{
		{
			name:  "config",
			usage: printConfigUsage,
			want: []string{
				"config <subcommand> [options]",
				"Manages application configuration with versioning and scope support.",
				"Reading Configuration:",
				"Modifying Configuration:",
				"Version Control:",
				"get [filter]",
				"set <path> <value>",
				"list [scope]",
				"-scope string",
				"-format string",
				"-desc string",
				"-zero",
				"-runtime",
				"ripc config dump",
			},
		},
		{
			name:  "app",
			usage: printAppUsage,
			want: []string{
				"app <subcommand> [options]",
				"Manages the application lifecycle.",
				"create",
				"Create a new application instance",
			},
			notWant: []string{"Options:"},
		},
		{
			name:  "job",
			usage: printJobUsage,
			want: []string{
				"job <subcommand> [options]",
				"Manages background jobs.",
				"list [limit]",
				"rm <job_id>",
			},
			notWant: []string{"Options:"},
		},
		{
			name:  "log",
			usage: printLogUsage,
			want: []string{
				"log <subcommand> [options]",
				"Manages the logger database.",
				"init",
				"Initialize the log database and schema",
			},
			notWant: []string{"Options:"},
		},
		{
			name:  "config get",
			usage: printConfigGetUsage,
			want: []string{
				"config get [options] [filter]",
				"Gets configuration values by path.",
				"Arguments:",
				"filter",
				"Optional substring filter on configuration paths",
				"Options:",
				"-scope string",
				"ripc config get --scope my-app server.port",
			},
		},
		{
			name:  "config paths",
			usage: printConfigPathsUsage,
			want: []string{
				"config paths [options] [filter]",
				"Lists all keys in the configuration.",
				"Arguments:",
				"filter",
				"Options:",
				"-scope string",
				"ripc config paths server",
			},
		},
		{
			name:  "config dump",
			usage: printConfigDumpUsage,
			want: []string{
				"config dump [options]",
				"Dumps the configuration.",
				"Options:",
				"-scope string",
				"-zero",
				"-runtime",
				"ripc config dump --zero",
			},
		},
		{
			name:  "config scopes",
			usage: printConfigScopesUsage,
			want: []string{
				"config scopes",
				"Lists all configuration scopes.",
				"ripc config scopes",
			},
			notWant: []string{"Options:"},
		},
		{
			name:  "config set",
			usage: printConfigSetUsage,
			want: []string{
				"config set [options] <path> <value>",
				"Sets a configuration value at a specified path.",
				"Arguments:",
				"path",
				"Configuration path to set",
				"value",
				"Value to set",
				"Options:",
				"-scope string",
				"-format string",
				"-desc string",
				"ripc config set server.host localhost",
			},
		},
		{
			name:  "config save",
			usage: printConfigSaveUsage,
			want: []string{
				"config save [options] <file>",
				"Saves file contents to the configuration.",
				"Arguments:",
				"file",
				"Path to the configuration file to save",
				"Options:",
				"-scope string",
				"-format string",
				"-desc string",
				"ripc config save config.toml",
			},
		},
		{
			name:  "config scaffold",
			usage: printConfigScaffoldUsage,
			want: []string{
				"config scaffold [options] <type> <key>",
				"Scaffolds a new configuration entry",
				"Arguments:",
				"type",
				"key",
				"Scaffold Types:",
				"backup",
				"oauth2",
				"Options:",
				"-scope string",
				"-desc string",
				"ripc config scaffold backup app_db",
			},
		},
		{
			name:  "config migrate",
			usage: printConfigMigrateUsage,
			want: []string{
				"config migrate",
				"Migrates configuration to the current framework version.",
				"ripc config migrate",
			},
			notWant: []string{"Options:"},
		},
		{
			name:  "config list",
			usage: printConfigListUsage,
			want: []string{
				"config list [scope]",
				"Lists configuration versions.",
				"Arguments:",
				"scope",
				"Optional scope to filter versions by",
				"ripc config list my-app",
			},
			notWant: []string{"Options:"},
		},
		{
			name:  "config diff",
			usage: printConfigDiffUsage,
			want: []string{
				"config diff [options] <generation>",
				"Compares a configuration generation with the latest.",
				"Arguments:",
				"generation",
				"Generation number to compare against the latest",
				"Options:",
				"-scope string",
				"ripc config diff 3",
			},
		},
		{
			name:  "config rollback",
			usage: printConfigRollbackUsage,
			want: []string{
				"config rollback [options] <generation>",
				"Restores a previous configuration version.",
				"Arguments:",
				"generation",
				"Generation number to restore",
				"Options:",
				"-scope string",
				"ripc config rollback 3",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			tc.usage(&buf)

			output := buf.String()
			if output == "" {
				t.Fatal("expected non-empty help output")
			}
			for _, want := range tc.want {
				if !strings.Contains(output, want) {
					t.Errorf("expected output to contain %q, but it did not.\n\nGot:\n%s", want, output)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(output, notWant) {
					t.Errorf("expected output not to contain %q, but it did.\n\nGot:\n%s", notWant, output)
				}
			}
		})
	}
}
