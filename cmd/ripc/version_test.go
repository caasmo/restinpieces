package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestHandleVersionCommand(t *testing.T) {
	oldTag := BuildTag
	oldCommit := BuildCommit
	BuildTag = "v0.0.0-test"
	BuildCommit = "abc1234"
	defer func() {
		BuildTag = oldTag
		BuildCommit = oldCommit
	}()

	var out bytes.Buffer
	var errBuf bytes.Buffer
	ui := UI{Out: &out, Err: &errBuf}

	err := handleVersionCommand(nil, ui)
	if err != nil {
		t.Fatalf("handleVersionCommand() returned unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	want := "v0.0.0-test abc1234"
	if got != want {
		t.Errorf("handleVersionCommand() = %q, want %q", got, want)
	}
}

func TestHandleVersionCommand_TooManyArguments(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer
	ui := UI{Out: &out, Err: &errBuf}

	err := handleVersionCommand([]string{"extra"}, ui)
	if !errors.Is(err, ErrTooManyArguments) {
		t.Errorf("handleVersionCommand() error = %v, want error wrapping %v", err, ErrTooManyArguments)
	}
}

func TestPrintVersionUsage(t *testing.T) {
	var buf bytes.Buffer
	printVersionUsage(&buf)
	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty help output")
	}
	for _, want := range []string{"version", "Prints the ripc build tag and commit.", "ripc version"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected output to contain %q, but it did not.\n\nGot:\n%s", want, output)
		}
	}
}
