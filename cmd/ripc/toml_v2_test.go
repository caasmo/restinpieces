package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTomlValueFromText_Values(t *testing.T) {
	tests := []struct {
		name string
		text string
		want interface{}
	}{
		{name: "QuotedString", text: `"localhost:9999"`, want: "localhost:9999"},
		{name: "UnquotedWord", text: "hello", want: "hello"},
		{name: "AddressWithoutQuotes", text: ":8080", want: ":8080"},
		{name: "Integer", text: "500", want: int64(500)},
		{name: "Float", text: "1.5", want: 1.5},
		{name: "Bool", text: "true", want: true},
		{name: "Array", text: "[1, 2]", want: []interface{}{int64(1), int64(2)}},
		{name: "EscapedString", text: `"a\nb"`, want: "a\nb"},
		{name: "Empty", text: "", want: ""},
		{name: "TextWithSpaces", text: "new value from file", want: "new value from file"},
		{name: "BrokenTomlSyntax", text: "[server", want: "[server"},
		{name: "MultilineText", text: "line1\nline2", want: "line1\nline2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tomlValueFromText(tt.text)
			if err != nil {
				t.Fatalf("tomlValueFromText(%q) failed: %v", tt.text, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("tomlValueFromText(%q) = %#v, want %#v", tt.text, got, tt.want)
			}
		})
	}
}

func TestTomlValueFromFile_NotAFileArgument(t *testing.T) {
	got, isFile, err := tomlValueFromFile("hello")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isFile {
		t.Error("expected isFile false for a plain value")
	}
	if got != nil {
		t.Errorf("expected nil value, got %#v", got)
	}
}

func TestTomlValueFromFile_Contents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value.txt")
	content := "line1\nline2\n"
	writeErr := os.WriteFile(path, []byte(content), 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write test file: %v", writeErr)
	}

	got, isFile, err := tomlValueFromFile("@" + path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isFile {
		t.Error("expected isFile true for an @ argument")
	}
	if got != content {
		t.Errorf("file value = %q, want %q", got, content)
	}
}

// TestTomlValueFromFile_ContentIsNotParsed verifies that file contents are
// taken as a plain string, even when they look like TOML.
func TestTomlValueFromFile_ContentIsNotParsed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value.txt")
	writeErr := os.WriteFile(path, []byte("42"), 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write test file: %v", writeErr)
	}

	got, isFile, err := tomlValueFromFile("@" + path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isFile {
		t.Error("expected isFile true for an @ argument")
	}
	if got != "42" {
		t.Errorf("file value = %#v, want the string \"42\"", got)
	}
}

func TestTomlValueFromFile_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")

	_, isFile, err := tomlValueFromFile("@" + path)

	if !errors.Is(err, ErrReadFile) {
		t.Fatalf("expected error to wrap ErrReadFile, got %v", err)
	}
	if !isFile {
		t.Error("expected isFile true for an @ argument")
	}
}
