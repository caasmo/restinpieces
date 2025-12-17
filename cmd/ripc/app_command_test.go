package main

import (
	"errors"
	"io"
	"reflect"
	"testing"
)

func TestParseAppSubcommand(t *testing.T) {
	testCases := []struct {
		name         string
		args         []string
		expectedCmd  string
		expectedArgs []string
		expectedErr  error
	}{
		// General Failure
		{
			name:        "UnknownSubcommand",
			args:        []string{"nonexistent-command"},
			expectedErr: ErrUnknownAppSubcommand,
		},

		// 'create' subcommand
		{
			name:         "CreateSuccess",
			args:         []string{"create"},
			expectedCmd:  "create",
			expectedArgs: nil,
			expectedErr:  nil,
		},
		{
			name:        "CreateTooManyArgs",
			args:        []string{"create", "extra-arg"},
			expectedErr: ErrTooManyArguments,
		},
		{
			name:        "CreateWithInvalidFlag",
			args:        []string{"create", "--invalid-flag"},
			expectedErr: ErrInvalidFlag,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, args, err := parseAppSubcommand(io.Discard, tc.args)

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
