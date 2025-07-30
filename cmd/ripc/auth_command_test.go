package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseAuthSubcommand(t *testing.T) {
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
			expectedErr: ErrUnknownAuthSubcommand,
		},

		// 'rotate-jwt-secrets' subcommand
		{
			name:         "RotateJwtSecretsSuccess",
			args:         []string{"rotate-jwt-secrets"},
			expectedCmd:  "rotate-jwt-secrets",
			expectedArgs: nil,
			expectedErr:  nil,
		},
		{
			name:        "RotateJwtSecretsTooManyArgs",
			args:        []string{"rotate-jwt-secrets", "extra"},
			expectedErr: ErrTooManyArguments,
		},

		// 'add-oauth2' subcommand
		{
			name:         "AddOAuth2Success",
			args:         []string{"add-oauth2", "google"},
			expectedCmd:  "add-oauth2",
			expectedArgs: []string{"google"},
			expectedErr:  nil,
		},
		{
			name:        "AddOAuth2MissingArgument",
			args:        []string{"add-oauth2"},
			expectedErr: ErrMissingArgument,
		},
		{
			name:        "AddOAuth2TooManyArguments",
			args:        []string{"add-oauth2", "google", "extra"},
			expectedErr: ErrMissingArgument, // Should be ErrTooManyArguments after change
		},

		// 'rm-oauth2' subcommand
		{
			name:         "RmOAuth2Success",
			args:         []string{"rm-oauth2", "github"},
			expectedCmd:  "rm-oauth2",
			expectedArgs: []string{"github"},
			expectedErr:  nil,
		},
		{
			name:        "RmOAuth2MissingArgument",
			args:        []string{"rm-oauth2"},
			expectedErr: ErrMissingArgument,
		},
		{
			name:        "RmOAuth2TooManyArguments",
			args:        []string{"rm-oauth2", "github", "extra"},
			expectedErr: ErrMissingArgument, // Should be ErrTooManyArguments after change
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, args, err := parseAuthSubcommand(tc.args)

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
