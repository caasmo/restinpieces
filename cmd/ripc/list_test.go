package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPrintConfigList_Success(t *testing.T) {
	configs := [][2]string{
		{"scope-a", "content-a"},
		{"scope-b", "content-b"},
		{"scope-a", "content-c"},
	}
	db := newTestAppDb(t)
	if err := db.createSchemas(); err != nil {
		t.Fatalf("createSchemas failed: %v", err)
	}
	for _, config := range configs {
		if err := db.InsertConfig(config[0], []byte(config[1]), "json", ""); err != nil {
			t.Fatalf("failed to insert config for scope %q: %v", config[0], err)
		}
	}

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	count, err := printConfigList(ui, db, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 items, got %d", count)
	}

	output := stdout.String()
	if !strings.Contains(output, "scope-a") || !strings.Contains(output, "scope-b") {
		t.Errorf("output does not contain expected scopes: %s", output)
	}
}

func TestPrintConfigList_Success_NoItems(t *testing.T) {
	db := newTestAppDb(t)
	if err := db.createSchemas(); err != nil {
		t.Fatalf("createSchemas failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	count, err := printConfigList(ui, db, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 items, got %d", count)
	}
}

func TestPrintConfigList_Failure_DbConnectionError(t *testing.T) {
	db := newTestAppDb(t)
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close database pool: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	_, err := printConfigList(ui, db, "")

	if !errors.Is(err, ErrDbConnection) {
		t.Errorf("expected ErrDbConnection, got %v", err)
	}
}

func TestPrintConfigList_Failure_QueryError(t *testing.T) {
	// No schema is applied, so the app_config table does not exist and query
	// preparation fails.
	db := newTestAppDb(t)

	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}
	_, err := printConfigList(ui, db, "")

	if err == nil {
		t.Fatal("expected an error, but got nil")
	}

	if !errors.Is(err, ErrQueryPrepare) {
		t.Errorf("expected ErrQueryPrepare, got %v", err)
	}
}
