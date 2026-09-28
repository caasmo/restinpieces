package main

import (
	"testing"

	toml "github.com/pelletier/go-toml"
)

func TestTomlTableOf_Table(t *testing.T) {
	table, err := toml.TreeFromMap(map[string]interface{}{})
	if err != nil {
		t.Fatalf("failed to create a table: %v", err)
	}

	got, ok := tomlTableOf(table)
	if !ok {
		t.Fatal("expected a table to be recognized")
	}
	if got == nil {
		t.Fatal("expected the table value")
	}
}

func TestTomlTableOf_PlainValue(t *testing.T) {
	got, ok := tomlTableOf("plain")
	if ok {
		t.Fatal("expected a plain value not to be a table")
	}
	if got != nil {
		t.Fatal("expected no table for a plain value")
	}
}

func TestTomlArrayOf_Array(t *testing.T) {
	array, ok := tomlArrayOf([]interface{}{"GPTBot"})
	if !ok {
		t.Fatal("expected an array to be recognized")
	}
	if len(array) != 1 || array[0] != "GPTBot" {
		t.Fatalf("expected [GPTBot], got %v", array)
	}
}

func TestTomlArrayOf_PlainValue(t *testing.T) {
	array, ok := tomlArrayOf("plain")
	if ok {
		t.Fatal("expected a plain value not to be an array")
	}
	if array != nil {
		t.Fatal("expected no array for a plain value")
	}
}
