package main

import (
	"errors"
	"testing"

	"github.com/caasmo/restinpieces/config"
)

func TestParseConfigSubcommand(t *testing.T) {
	// Test unknown subcommand - now handled in handleConfigCommand
	t.Run("UnknownSubcommand", func(t *testing.T) {
		// This is now handled in handleConfigCommand's default case
		// We'll test it through integration tests
	})

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
	testInitParsing(t)
}

func testSetParsing(t *testing.T) {
	t.Run("SetSuccess", func(t *testing.T) {
		scope, format, desc, path, value, remainingArgs, err := parseSetArgs([]string{"--scope", "my-scope", "--desc", "My Change", "server.addr", ":8081"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != "my-scope" {
			t.Errorf("expected scope 'my-scope', got %q", scope)
		}
		if format != "toml" {
			t.Errorf("expected format 'toml', got %q", format)
		}
		if desc != "My Change" {
			t.Errorf("expected desc 'My Change', got %q", desc)
		}
		if path != "server.addr" {
			t.Errorf("expected path 'server.addr', got %q", path)
		}
		if value != ":8081" {
			t.Errorf("expected value ':8081', got %q", value)
		}
		if len(remainingArgs) != 0 {
			t.Errorf("expected no remaining args, got %v", remainingArgs)
		}
	})

	t.Run("SetMissingValue", func(t *testing.T) {
		_, _, _, _, _, _, err := parseSetArgs([]string{"server.addr"})
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
		scope, err := parseListArgs([]string{"test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != "test" {
			t.Errorf("expected scope 'test', got %q", scope)
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
		scope, filter, err := parsePathsArgs([]string{"--scope", "test", "filter"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != "test" {
			t.Errorf("expected scope 'test', got %q", scope)
		}
		if filter != "filter" {
			t.Errorf("expected filter 'filter', got %q", filter)
		}
	})

	t.Run("PathsTooManyArgs", func(t *testing.T) {
		_, _, err := parsePathsArgs([]string{"filter", "extra"})
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
		scope, err := parseDumpArgs([]string{"--scope", "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != "test" {
			t.Errorf("expected scope 'test', got %q", scope)
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
		scope, generation, err := parseDiffArgs([]string{"123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != config.ScopeApplication {
			t.Errorf("expected scope %q, got %q", config.ScopeApplication, scope)
		}
		if generation != 123 {
			t.Errorf("expected generation 123, got %d", generation)
		}
	})

	t.Run("DiffNotANumber", func(t *testing.T) {
		_, _, err := parseDiffArgs([]string{"abc"})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrNotANumber) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrNotANumber, err)
		}
	})

	t.Run("DiffMissingArgument", func(t *testing.T) {
		_, _, err := parseDiffArgs([]string{})
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
		scope, generation, err := parseRollbackArgs([]string{"--scope", "custom", "42"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != "custom" {
			t.Errorf("expected scope 'custom', got %q", scope)
		}
		if generation != 42 {
			t.Errorf("expected generation 42, got %d", generation)
		}
	})

	t.Run("RollbackTooManyArgs", func(t *testing.T) {
		_, _, err := parseRollbackArgs([]string{"42", "extra"})
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
		scope, format, desc, filename, err := parseSaveArgs([]string{"--scope", "test", "file.toml"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != "test" {
			t.Errorf("expected scope 'test', got %q", scope)
		}
		if format != "" {
			t.Errorf("expected empty format, got %q", format)
		}
		if desc != "" {
			t.Errorf("expected empty desc, got %q", desc)
		}
		if filename != "file.toml" {
			t.Errorf("expected filename 'file.toml', got %q", filename)
		}
	})

	t.Run("SaveSuccessWithAllFlags", func(t *testing.T) {
		scope, format, desc, filename, err := parseSaveArgs([]string{"--scope", "test", "--format", "json", "--desc", "my description", "file.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != "test" {
			t.Errorf("expected scope 'test', got %q", scope)
		}
		if format != "json" {
			t.Errorf("expected format 'json', got %q", format)
		}
		if desc != "my description" {
			t.Errorf("expected desc 'my description', got %q", desc)
		}
		if filename != "file.json" {
			t.Errorf("expected filename 'file.json', got %q", filename)
		}
	})

	t.Run("SaveMissingArgument", func(t *testing.T) {
		_, _, _, _, err := parseSaveArgs([]string{})
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
		scope, filter, err := parseGetArgs([]string{"--scope", "test", "filter"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != "test" {
			t.Errorf("expected scope 'test', got %q", scope)
		}
		if filter != "filter" {
			t.Errorf("expected filter 'filter', got %q", filter)
		}
	})
}

func testInitParsing(t *testing.T) {
	t.Run("InitSuccess", func(t *testing.T) {
		err := parseInitArgs([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("InitTooManyArgs", func(t *testing.T) {
		err := parseInitArgs([]string{"extra"})
		if err == nil {
			t.Fatal("expected error, but got nil")
		}
		if !errors.Is(err, ErrTooManyArguments) {
			t.Fatalf("expected error to wrap %v, but got %v", ErrTooManyArguments, err)
		}
	})
}
