package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchUserAgents(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"SemrushBot":{"operator":"Semrush"},"GPTBot":{"operator":"OpenAI"}}`))
		}))
		defer server.Close()

		agents, err := fetchUserAgents(server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(agents) != 2 {
			t.Fatalf("expected 2 user agents, got %v", agents)
		}
		found := make(map[string]struct{}, len(agents))
		for _, agent := range agents {
			found[agent] = struct{}{}
		}
		for _, want := range []string{"GPTBot", "SemrushBot"} {
			_, ok := found[want]
			if !ok {
				t.Errorf("expected %q in list, got %v", want, agents)
			}
		}
	})

	t.Run("StatusError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		_, err := fetchUserAgents(server.URL)
		if !errors.Is(err, ErrUserAgentFetch) {
			t.Fatalf("expected error to wrap ErrUserAgentFetch, got %v", err)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`not json`))
		}))
		defer server.Close()

		_, err := fetchUserAgents(server.URL)
		if !errors.Is(err, ErrUserAgentParse) {
			t.Fatalf("expected error to wrap ErrUserAgentParse, got %v", err)
		}
	})

	t.Run("EmptyUserAgents", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		agents, err := fetchUserAgents(server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(agents) != 0 {
			t.Errorf("expected no user agents, got %v", agents)
		}
	})
}
