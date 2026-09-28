package main

import (
	"errors"
	"testing"
)

func TestValidateTomlAsConfig_Valid(t *testing.T) {
	data := []byte("[server]\n  addr = \":8080\"\n")

	err := validateTomlAsConfig(data, "server.addr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateTomlAsConfig_Invalid(t *testing.T) {
	data := []byte("server = \"oops\"\n")

	err := validateTomlAsConfig(data, "server")
	if !errors.Is(err, ErrConfigUnmarshal) {
		t.Fatalf("expected error to wrap ErrConfigUnmarshal, got %v", err)
	}
}
