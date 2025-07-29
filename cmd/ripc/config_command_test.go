package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

func TestParseConfigSubcommand(t *testing.T) {
	testCases := []struct {
		name         string
		args         []string
		expectedCmd  string
		expectedArgs []string
		expectedErr  error
	}{
		// General Failures
		{
			name:        "UnknownSubcommand",
			args:        []string{"nonexistent-command"},
			expectedErr: ErrUnknownSubcommand,
		},

		// 'set' subcommand
		{
			name:         "SetSuccess",
			args:         []string{"set", "--scope", "my-scope", "--desc", "My Change", "server.addr", ":8081"},
			expectedCmd:  "set",
			expectedArgs: []string{"my-scope", "toml", "My Change", "server.addr", ":8081"},
			expectedErr:  nil,
		},
		{
			name:        "SetMissingValue",
			args:        []string{"set", "server.addr"},
			expectedErr: ErrMissingArgument,
		},

		// 'scopes' subcommand
		{
			name:        "ScopesSuccess",
			args:        []string{"scopes"},
			expectedCmd: "scopes",
			expectedErr: nil,
		},
		{
			name:        "ScopesTooManyArgs",
			args:        []string{"scopes", "extra"},
			expectedErr: ErrTooManyArguments,
		},

		// 'diff' subcommand
		{
			name:         "DiffSuccess",
			args:         []string{"diff", "123"},
			expectedCmd:  "diff",
			expectedArgs: []string{config.ScopeApplication, "123"},
			expectedErr:  nil,
		},
		{
			name:        "DiffNotANumber",
			args:        []string{"diff", "abc"},
			expectedErr: ErrNotANumber,
		},
		{
			name:        "DiffMissingArgument",
			args:        []string{"diff"},
			expectedErr: ErrMissingArgument,
		},

		// 'rollback' subcommand
		{
			name:         "RollbackSuccessWithScope",
			args:         []string{"rollback", "--scope", "custom", "42"},
			expectedCmd:  "rollback",
			expectedArgs: []string{"custom", "42"},
			expectedErr:  nil,
		},
		{
			name:        "RollbackTooManyArgs",
			args:        []string{"rollback", "42", "extra"},
			expectedErr: ErrTooManyArguments,
		},

		// 'init' subcommand
		{
			name:        "InitSuccess",
			args:        []string{"init"},
			expectedCmd: "init",
			expectedErr: nil,
		},
		{
			name:        "InitTooManyArgs",
			args:        []string{"init", "extra"},
			expectedErr: ErrTooManyArguments,
		},

		// 'paths' subcommand
		{
			name:         "PathsSuccess",
			args:         []string{"paths", "--scope", "test", "filter"},
			expectedCmd:  "paths",
			expectedArgs: []string{"test", "filter"},
			expectedErr:  nil,
		},
		{
			name:        "PathsTooManyArgs",
			args:        []string{"paths", "filter", "extra"},
			expectedErr: ErrTooManyArguments,
		},

		// 'dump' subcommand
		{
			name:         "DumpSuccess",
			args:         []string{"dump", "--scope", "test"},
			expectedCmd:  "dump",
			expectedArgs: []string{"test"},
			expectedErr:  nil,
		},
		{
			name:        "DumpTooManyArgs",
			args:        []string{"dump", "extra"},
			expectedErr: ErrTooManyArguments,
		},

		// 'get' subcommand
		{
			name:         "GetSuccess",
			args:         []string{"get", "--scope", "test", "filter"},
			expectedCmd:  "get",
			expectedArgs: []string{"test", "filter"},
			expectedErr:  nil,
		},

		// 'save' subcommand
		{
			name:         "SaveSuccess",
			args:         []string{"save", "--scope", "test", "file.toml"},
			expectedCmd:  "save",
			expectedArgs: []string{"test", "file.toml"},
			expectedErr:  nil,
		},
		{
			name:        "SaveMissingArgument",
			args:        []string{"save"},
			expectedErr: ErrMissingArgument,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, args, err := parseConfigSubcommand(tc.args)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error, but got nil")
				}
				if !errors.Is(err, tc.expectedErr) {
					t.Fatalf("expected error to wrap %v, but got %v", tc.expectedErr, err)
				}
				return // Test ends here for error cases
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cmd != tc.expectedCmd {
				t.Errorf("expected subcommand %q, but got %q", tc.expectedCmd, cmd)
			}

			if !reflect.DeepEqual(args, tc.expectedArgs) {
				t.Errorf("expected args %v, but got %v", tc.expectedArgs, args)
			}
		})
	}
}
