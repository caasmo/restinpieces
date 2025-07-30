package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseLogSubcommand(t *testing.T) {
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
			expectedErr: ErrUnknownLogSubcommand,
		},

		// 'init' subcommand
		{
			name:         "InitSuccess",
			args:         []string{"init"},
			expectedCmd:  "init",
			expectedArgs: nil,
			expectedErr:  nil,
		},
		{
			name:        "InitTooManyArgs",
			args:        []string{"init", "extra-arg"},
			expectedErr: ErrTooManyArguments,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, args, err := parseLogSubcommand(tc.args)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatal("expected error, but got nil")
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
