package main

import (
	"errors"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

func TestParseArgs(t *testing.T) {
	// Test individual parsing functions
	testSetParsing(t)
	testScopesParsing(t)
	testListParsing(t)
	testPathsParsing(t)
	testDumpParsing(t)
	testDiffParsing(t)
	testRollbackParsing(t)
	testSaveParsing(t)
	testGetParsing(t)
	testMigrateParsing(t)
}

func testSetParsing(t *testing.T) {
	t.Run("SetSuccess", func(t *testing.T) {
		opts, err := parseSetArgs([]string{"--scope", "my-scope", "--desc", "My Change", "server.addr", ":8081"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Scope != "my-scope" {
			t.Errorf("expected scope 'my-scope', got %q", opts.Scope)
		}
		if opts.Format != "toml" {
			t.Errorf("expected format 'toml', got %q", opts.Format)
		}
		if opts.Desc != "My Change" {
			t.Errorf("expected desc 'My Change', got %q", opts.Desc)
		}
		if opts.Path != "server.addr" {
			t.Errorf("expected path 'server.addr', got %q", opts.Path)
		}
		if opts.Value != ":8081" {
			t.Errorf("expected value ':8081', got %q", opts.Value)
		}
	})

	t.Run("SetMissingValue", func(t *testing.T) {
		_, err := parseSetArgs([]string{"server.addr"})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrMissingArgument) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrMissingArgument, err)
		}
	})
}

func testScopesParsing(t *testing.T) {
	t.Run("ScopesSuccess", func(t *testing.T) {
		err := parseScopesArgs([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ScopesTooManyArgs", func(t *testing.T) {
		err := parseScopesArgs([]string{"extra"})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrTooManyArguments) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrTooManyArguments, err)
		}
	})
}

func testListParsing(t *testing.T) {
	// Note: list command doesn't have flags, just optional scope argument
	t.Run("ListSuccess", func(t *testing.T) {
		opts, err := parseListArgs([]string{"test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Scope != "test" {
			t.Errorf("expected scope 'test', got %q", opts.Scope)
		}
	})

	t.Run("ListTooManyArgs", func(t *testing.T) {
		_, err := parseListArgs([]string{"scope1", "scope2"})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrTooManyArguments) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrTooManyArguments, err)
		}
	})
}

func testPathsParsing(t *testing.T) {
	t.Run("PathsSuccess", func(t *testing.T) {
		opts, err := parsePathsArgs([]string{"--scope", "test", "filter"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Scope != "test" {
			t.Errorf("expected scope 'test', got %q", opts.Scope)
		}
		if opts.Filter != "filter" {
			t.Errorf("expected filter 'filter', got %q", opts.Filter)
		}
	})

	t.Run("PathsTooManyArgs", func(t *testing.T) {
		_, err := parsePathsArgs([]string{"filter", "extra"})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrTooManyArguments) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrTooManyArguments, err)
		}
	})
}

func testDumpParsing(t *testing.T) {
	t.Run("DumpSuccess", func(t *testing.T) {
		opts, err := parseDumpArgs([]string{"--scope", "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Scope != "test" {
			t.Errorf("expected scope 'test', got %q", opts.Scope)
		}
		if opts.Zero {
			t.Errorf("expected zero to be false by default")
		}
		if opts.Runtime {
			t.Errorf("expected runtime to be false by default")
		}
	})

	t.Run("DumpZeroSuccess", func(t *testing.T) {
		opts, err := parseDumpArgs([]string{"--zero"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !opts.Zero {
			t.Errorf("expected zero to be true when flag is set")
		}
		if opts.Runtime {
			t.Errorf("expected runtime to be false")
		}
	})

	t.Run("DumpEffectiveSuccess", func(t *testing.T) {
		opts, err := parseDumpArgs([]string{"--runtime"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Zero {
			t.Errorf("expected zero to be false")
		}
		if !opts.Runtime {
			t.Errorf("expected runtime to be true when flag is set")
		}
	})

	t.Run("DumpZeroAndEffectiveMutualExclusion", func(t *testing.T) {
		_, err := parseDumpArgs([]string{"--zero", "--runtime"})
		if err == nil {
			t.Fatal("expected error for mutually exclusive flags, got nil")
		}
		if !errors.Is(err, ErrInvalidFlag) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrInvalidFlag, err)
		}
	})

	t.Run("DumpTooManyArgs", func(t *testing.T) {
		_, err := parseDumpArgs([]string{"extra"})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrTooManyArguments) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrTooManyArguments, err)
		}
	})
}

func testDiffParsing(t *testing.T) {
	t.Run("DiffSuccess", func(t *testing.T) {
		opts, err := parseDiffArgs([]string{"123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Scope != config.ScopeApplication {
			t.Errorf("expected scope %q, got %q", config.ScopeApplication, opts.Scope)
		}
		if opts.Generation != 123 {
			t.Errorf("expected generation 123, got %d", opts.Generation)
		}
	})

	t.Run("DiffNotANumber", func(t *testing.T) {
		_, err := parseDiffArgs([]string{"abc"})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrNotANumber) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrNotANumber, err)
		}
	})

	t.Run("DiffMissingArgument", func(t *testing.T) {
		_, err := parseDiffArgs([]string{})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrMissingArgument) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrMissingArgument, err)
		}
	})
}

func testRollbackParsing(t *testing.T) {
	t.Run("RollbackSuccessWithScope", func(t *testing.T) {
		opts, err := parseRollbackArgs([]string{"--scope", "custom", "42"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Scope != "custom" {
			t.Errorf("expected scope 'custom', got %q", opts.Scope)
		}
		if opts.Generation != 42 {
			t.Errorf("expected generation 42, got %d", opts.Generation)
		}
	})

	t.Run("RollbackTooManyArgs", func(t *testing.T) {
		_, err := parseRollbackArgs([]string{"42", "extra"})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrTooManyArguments) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrTooManyArguments, err)
		}
	})
}

func testSaveParsing(t *testing.T) {
	t.Run("SaveSuccess", func(t *testing.T) {
		opts, err := parseSaveArgs([]string{"--scope", "test", "file.toml"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Scope != "test" {
			t.Errorf("expected scope 'test', got %q", opts.Scope)
		}
		if opts.Format != "" {
			t.Errorf("expected empty format, got %q", opts.Format)
		}
		if opts.Desc != "" {
			t.Errorf("expected empty desc, got %q", opts.Desc)
		}
		if opts.Filename != "file.toml" {
			t.Errorf("expected filename 'file.toml', got %q", opts.Filename)
		}
	})

	t.Run("SaveSuccessWithAllFlags", func(t *testing.T) {
		opts, err := parseSaveArgs([]string{"--scope", "test", "--format", "json", "--desc", "my description", "file.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Scope != "test" {
			t.Errorf("expected scope 'test', got %q", opts.Scope)
		}
		if opts.Format != "json" {
			t.Errorf("expected format 'json', got %q", opts.Format)
		}
		if opts.Desc != "my description" {
			t.Errorf("expected desc 'my description', got %q", opts.Desc)
		}
		if opts.Filename != "file.json" {
			t.Errorf("expected filename 'file.json', got %q", opts.Filename)
		}
	})

	t.Run("SaveMissingArgument", func(t *testing.T) {
		_, err := parseSaveArgs([]string{})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrMissingArgument) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrMissingArgument, err)
		}
	})
}

func testGetParsing(t *testing.T) {
	t.Run("GetSuccess", func(t *testing.T) {
		opts, err := parseGetArgs([]string{"--scope", "test", "filter"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Scope != "test" {
			t.Errorf("expected scope 'test', got %q", opts.Scope)
		}
		if opts.Filter != "filter" {
			t.Errorf("expected filter 'filter', got %q", opts.Filter)
		}
	})
}

func testMigrateParsing(t *testing.T) {
	t.Run("MigrateSuccess", func(t *testing.T) {
		err := parseMigrateArgs([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("MigrateTooManyArgs", func(t *testing.T) {
		err := parseMigrateArgs([]string{"extra"})
		if err == nil {
			t.Fatal("expected error, but did not")
		}
		if !errors.Is(err, ErrTooManyArguments) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrTooManyArguments, err)
		}
	})
}
