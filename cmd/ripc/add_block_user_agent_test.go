package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestAddValue_UserAgent_Append(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_user_agent.agents", "SemrushBot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tree := getUpdateTreeFromStore(t, mockStore, scope)
	raw, ok := tree.Get("block_user_agent.agents").([]interface{})
	if !ok {
		t.Fatalf("expected block_user_agent.agents to be a list, got %T", tree.Get("block_user_agent.agents"))
	}

	got := make([]string, 0, len(raw))
	for _, item := range raw {
		str, ok := item.(string)
		if !ok {
			t.Fatalf("expected list item to be string, got %T", item)
		}
		got = append(got, str)
	}

	for _, want := range []string{"GPTBot", "SemrushBot"} {
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

func TestAddValue_UserAgent_DuplicateSkip(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_user_agent.agents", "GPTBot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tree := getUpdateTreeFromStore(t, mockStore, scope)
	raw, ok := tree.Get("block_user_agent.agents").([]interface{})
	if !ok {
		t.Fatalf("expected block_user_agent.agents to be a list, got %T", tree.Get("block_user_agent.agents"))
	}

	if len(raw) != 1 {
		t.Fatalf("expected list unchanged with 1 item, got %v", raw)
	}
	if raw[0] != "GPTBot" {
		t.Errorf("expected %q, got %v", "GPTBot", raw[0])
	}
}

func TestAddValue_UserAgent_EmptyValue(t *testing.T) {
	scope := "app"
	mockStore := NewMockUpdateSecureStore(map[string][]byte{scope: []byte(addTestConf)})
	var stdout, stderr bytes.Buffer
	ui := UI{Out: &stdout, Err: &stderr}

	err := addValue(ui, mockStore, scope, "", "block_user_agent.agents", "")
	if err == nil {
		t.Fatal("expected error, but got nil")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("expected empty value error, got %v", err)
	}
}
