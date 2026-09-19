package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestAddValue_BlockHost_Append(t *testing.T) {
	scope := "app"
	mockStore := NewMockAddSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_host.allowed_hosts", "example.org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tree := getAddTreeFromStore(t, mockStore, scope)
	raw, ok := tree.Get("block_host.allowed_hosts").([]interface{})
	if !ok {
		t.Fatalf("expected block_host.allowed_hosts to be a list, got %T", tree.Get("block_host.allowed_hosts"))
	}

	got := make([]string, 0, len(raw))
	for _, item := range raw {
		str, ok := item.(string)
		if !ok {
			t.Fatalf("expected list item to be string, got %T", item)
		}
		got = append(got, str)
	}

	for _, want := range []string{"example.com", "example.org"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in %v", want, got)
		}
	}
}

func TestAddValue_BlockHost_DuplicateSkip(t *testing.T) {
	scope := "app"
	mockStore := NewMockAddSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_host.allowed_hosts", "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tree := getAddTreeFromStore(t, mockStore, scope)
	raw, ok := tree.Get("block_host.allowed_hosts").([]interface{})
	if !ok {
		t.Fatalf("expected block_host.allowed_hosts to be a list, got %T", tree.Get("block_host.allowed_hosts"))
	}

	if len(raw) != 1 {
		t.Fatalf("expected list unchanged with 1 item, got %v", raw)
	}
	if raw[0] != "example.com" {
		t.Errorf("expected %q, got %v", "example.com", raw[0])
	}
}

func TestAddValue_BlockHost_EmptyValue(t *testing.T) {
	scope := "app"
	mockStore := NewMockAddSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_host.allowed_hosts", "")
	if err == nil {
		t.Fatal("expected error, but got nil")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("expected empty value error, got %v", err)
	}
}
