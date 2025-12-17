package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/caasmo/restinpieces"
)

func TestRun(t *testing.T) {
	// Helper to create a dummy age key file that is syntactically valid
	createDummyAgeKeyFile := func(t *testing.T, dir string) string {
		t.Helper()
		path := filepath.Join(dir, "key.txt")
		// This is a syntactically valid, insecure private key for testing.
		key := "AGE-SECRET-KEY-17QYJ5A2SHMP6N252M3T4CE8M9E0Q3QYJ5A2SHMP6N252M3T4CE8M9E0Q3QY"
		if err := os.WriteFile(path, []byte(key), 0644); err != nil {
			t.Fatalf("Failed to create dummy age key file %s: %v", path, err)
		}
		return path
	}

	// Helper to create a valid, empty SQLite database file
	createDummyDB := func(t *testing.T, dir string) string {
		t.Helper()
		path := filepath.Join(dir, "test.db")
		pool, err := restinpieces.NewZombiezenPool(path)
		if err != nil {
			t.Fatalf("Failed to create dummy db: %v", err)
		}
		if err := pool.Close(); err != nil {
			t.Fatalf("Failed to close dummy db: %v", err)
		}
		return path
	}

	// Helper to create a simple dummy file for existence checks
	createDummyFile := func(t *testing.T, dir string) string {
		t.Helper()
		path := filepath.Join(dir, "test.db")
		if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
			t.Fatalf("Failed to create dummy file %s: %v", path, err)
		}
		return path
	}

	testCases := []struct {
		name        string
		setup       func(t *testing.T, dir string) []string
		expectedErr error
	}{
		{
			name: "MissingAgeKeyFlag",
			setup: func(t *testing.T, dir string) []string {
				return []string{"-dbpath", "dummy.db"}
			},
			expectedErr: ErrMissingFlag,
		},
		{
			name: "MissingDbPathFlag",
			setup: func(t *testing.T, dir string) []string {
				ageKeyPath := createDummyAgeKeyFile(t, dir)
				return []string{"-agekey", ageKeyPath}
			},
			expectedErr: ErrMissingFlag,
		},
		{
			name: "MissingCommand",
			setup: func(t *testing.T, dir string) []string {
				ageKeyPath := createDummyAgeKeyFile(t, dir)
				dbPath := createDummyDB(t, dir)
				return []string{"-agekey", ageKeyPath, "-dbpath", dbPath}
			},
			expectedErr: ErrMissingCommand,
		},
		{
			name: "UnknownCommand",
			setup: func(t *testing.T, dir string) []string {
				ageKeyPath := createDummyAgeKeyFile(t, dir)
				dbPath := createDummyDB(t, dir)
				return []string{"-agekey", ageKeyPath, "-dbpath", dbPath, "nonexistent-command"}
			},
			expectedErr: ErrUnknownCommand,
		},
		{
			name: "DBNotFoundForStandardCommand",
			setup: func(t *testing.T, dir string) []string {
				ageKeyPath := createDummyAgeKeyFile(t, dir)
				dbPath := filepath.Join(dir, "nonexistent.db") // Does not exist
				return []string{"-agekey", ageKeyPath, "-dbpath", dbPath, "config", "list"}
			},
			expectedErr: ErrDBNotFound,
		},
		{
			name: "DBAlreadyExistsForAppCreate",
			setup: func(t *testing.T, dir string) []string {
				ageKeyPath := createDummyAgeKeyFile(t, dir)
				dbPath := createDummyFile(t, dir) // Exists, can be any file
				return []string{"-agekey", ageKeyPath, "-dbpath", dbPath, "app", "create"}
			},
			expectedErr: ErrDBAlreadyExists,
		},
		{
			name: "SuccessPathWithHelp",
			setup: func(t *testing.T, dir string) []string {
				ageKeyPath := createDummyAgeKeyFile(t, dir)
				dbPath := createDummyDB(t, dir)
				return []string{"-agekey", ageKeyPath, "-dbpath", dbPath, "help"}
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			args := tc.setup(t, tempDir)
			var output bytes.Buffer

            err := run(args, &output)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error, but got nil")
				}
				if !errors.Is(err, tc.expectedErr) {
					t.Fatalf("expected error to wrap %v, but got %v", tc.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestRunDiscovery(t *testing.T) {
	// Helper to create a dummy age key file in the current working directory
	createDummyAgeKeyFile := func(t *testing.T, filename string) {
		t.Helper()
		key := "AGE-SECRET-KEY-17QYJ5A2SHMP6N252M3T4CE8M9E0Q3QYJ5A2SHMP6N252M3T4CE8M9E0Q3QY"
		if err := os.WriteFile(filename, []byte(key), 0644); err != nil {
			t.Fatalf("Failed to create dummy age key file %s: %v", filename, err)
		}
	}

	// Helper to create a valid, empty SQLite database file in the current working directory
	createDummyDB := func(t *testing.T, filename string) {
		t.Helper()
		pool, err := restinpieces.NewZombiezenPool(filename)
		if err != nil {
			t.Fatalf("Failed to create dummy db: %v", err)
		}
		if err := pool.Close(); err != nil {
			t.Fatalf("Failed to close dummy db: %v", err)
		}
	}

	testCases := []struct {
		name        string
		setup       func(t *testing.T) []string
		expectedErr error
	}{
		{
			name: "SuccessDiscoverAgeKeyAndDB",
			setup: func(t *testing.T) []string {
				createDummyAgeKeyFile(t, "age.key")
				createDummyDB(t, "app.db")
				return []string{"config", "list"}
			},
			expectedErr: nil,
		},
		{
			name: "SuccessDiscoverAgeKeyTxtAndDB",
			setup: func(t *testing.T) []string {
				createDummyAgeKeyFile(t, "age_key.txt")
				createDummyDB(t, "app.db")
				return []string{"config", "list"}
			},
			expectedErr: nil,
		},
		{
			name: "FailureMissingAgeKey",
			setup: func(t *testing.T) []string {
				createDummyDB(t, "app.db")
				return []string{"config", "list"}
			},
			expectedErr: ErrMissingFlag,
		},
		{
			name: "FailureMissingDB",
			setup: func(t *testing.T) []string {
				createDummyAgeKeyFile(t, "age.key")
				return []string{"config", "list"}
			},
			expectedErr: ErrDBNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current working directory: %v", err)
			}
			if err := os.Chdir(tempDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}
			defer os.Chdir(originalWD)

			args := tc.setup(t)
			var output bytes.Buffer

			err = run(args, &output)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error, but got nil")
				}
				if !errors.Is(err, tc.expectedErr) {
					t.Fatalf("expected error to wrap %v, but got %v", tc.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
