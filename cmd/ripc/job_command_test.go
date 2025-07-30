package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseJobSubcommand(t *testing.T) {
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
			expectedErr: ErrUnknownJobSubcommand,
		},

		// 'add-backup' subcommand
		{
			name:         "AddBackupSuccess",
			args:         []string{"add-backup", "--some-flag", "value"},
			expectedCmd:  "add-backup",
			expectedArgs: []string{"--some-flag", "value"},
			expectedErr:  nil,
		},

		// 'list' subcommand
		{
			name:         "ListSuccessNoArgs",
			args:         []string{"list"},
			expectedCmd:  "list",
			expectedArgs: []string{},
			expectedErr:  nil,
		},
		{
			name:         "ListSuccessWithLimit",
			args:         []string{"list", "10"},
			expectedCmd:  "list",
			expectedArgs: []string{"10"},
			expectedErr:  nil,
		},
		{
			name:        "ListTooManyArgs",
			args:        []string{"list", "10", "extra"},
			expectedErr: ErrTooManyArguments,
		},
		{
			name:        "ListLimitNotANumber",
			args:        []string{"list", "abc"},
			expectedErr: ErrNotANumber,
		},

		// 'rm' subcommand
		{
			name:         "RmSuccess",
			args:         []string{"rm", "123"},
			expectedCmd:  "rm",
			expectedArgs: []string{"123"},
			expectedErr:  nil,
		},
		{
			name:        "RmMissingArgument",
			args:        []string{"rm"},
			expectedErr: ErrMissingArgument,
		},
		{
			name:        "RmTooManyArgs",
			args:        []string{"rm", "123", "extra"},
			expectedErr: ErrTooManyArguments,
		},
		{
			name:        "RmIdNotANumber",
			args:        []string{"rm", "abc"},
			expectedErr: ErrNotANumber,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, args, err := parseJobSubcommand(tc.args)

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

			// Handle nil vs empty slice for expectedArgs
			if tc.expectedArgs == nil {
				tc.expectedArgs = []string{}
			}

			if !reflect.DeepEqual(args, tc.expectedArgs) {
				t.Errorf("expected args %v, but got %v", tc.expectedArgs, args)
			}
		})
	}
}
