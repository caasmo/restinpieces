package config

import "testing"

func TestOAuth2_Get(t *testing.T) {
	t.Parallel()
	entries := OAuth2{
		"my_google": {Name: "google"},
		"my_github": {Name: "github"},
	}

	entry, ok := entries.Get("github")
	if !ok {
		t.Fatal("Get() = false, want true")
	}
	if entry.Name != "github" {
		t.Errorf("Get() entry.Name = %q, want %q", entry.Name, "github")
	}

	if _, ok := entries.Get("facebook"); ok {
		t.Error("Get() = true for an unknown name, want false")
	}
}
