package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestListScopes_Success(t *testing.T) {
	initialScopes := []string{"scope-c", "scope-a", "scope-b", "scope-a"}
	db := newTestAppDb(t)
	if err := db.createSchemas(); err != nil {
		t.Fatalf("createSchemas failed: %v", err)
	}
	for _, scope := range initialScopes {
		if err := db.InsertConfig(scope, []byte("{}"), "json", ""); err != nil {
			t.Fatalf("failed to insert config for scope %q: %v", scope, err)
		}
	}

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := listScopes(ui, db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedOutput := "scope-a\nscope-b\nscope-c\n"
	if got := stdout.String(); got != expectedOutput {
		t.Errorf("expected output:\n%q\ngot:\n%q", expectedOutput, got)
	}
}

func TestListScopes_Success_NoScopes(t *testing.T) {
	db := newTestAppDb(t)
	if err := db.createSchemas(); err != nil {
		t.Fatalf("createSchemas failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := listScopes(ui, db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() > 0 {
		t.Errorf("expected empty output, but got: %q", stdout.String())
	}
}

func TestListScopes_Failure_DbConnectionError(t *testing.T) {
	db := newTestAppDb(t)
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close database pool: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := listScopes(ui, db)
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}

	if !errors.Is(err, ErrDbConnection) {
		t.Errorf("expected error to wrap ErrDbConnection, got %v", err)
	}
}
