package config

import (
	"errors"
	"regexp"
	"strconv"
	"testing"
)

func TestBuildUserAgentRegexp(t *testing.T) {
	t.Run("QuotesUserAgents", func(t *testing.T) {
		regExpr, err := BuildUserAgentRegexp([]string{"GPTBot", "Brightbot 1.0", "iaskspider/2.0"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := `(GPTBot|Brightbot 1\.0|iaskspider/2\.0)`
		if regExpr != want {
			t.Errorf("expected %q, got %q", want, regExpr)
		}
	})

	t.Run("Compiles", func(t *testing.T) {
		regExpr, err := BuildUserAgentRegexp([]string{"GPTBot", "SemrushBot"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := regexp.Compile(regExpr); err != nil {
			t.Errorf("expected the expression to compile, got error: %v", err)
		}
	})

	t.Run("EmptyUserAgents", func(t *testing.T) {
		_, err := BuildUserAgentRegexp(nil)
		if err == nil {
			t.Fatal("expected error when no user agents are given")
		}
	})

	t.Run("EmptyUserAgent", func(t *testing.T) {
		_, err := BuildUserAgentRegexp([]string{"GPTBot", ""})
		if err == nil {
			t.Fatal("expected error for an empty user agent")
		}
	})

	t.Run("TooLong", func(t *testing.T) {
		agents := make([]string, maxUserAgents+1)
		for i := range agents {
			agents[i] = "Bot" + strconv.Itoa(i)
		}

		_, err := BuildUserAgentRegexp(agents)
		if !errors.Is(err, ErrTooManyUserAgents) {
			t.Fatalf("expected error to wrap ErrTooManyUserAgents, got %v", err)
		}
	})
}

func TestParseUserAgentRegexp(t *testing.T) {
	t.Run("Members", func(t *testing.T) {
		stored, err := parseUserAgentRegexp(`(GPTBot|SemrushBot)`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(stored) != 2 {
			t.Fatalf("expected 2 user agents, got %v", stored)
		}
		for _, want := range []string{"GPTBot", "SemrushBot"} {
			_, ok := stored[want]
			if !ok {
				t.Errorf("expected %q in set, got %v", want, stored)
			}
		}
	})

	t.Run("EscapedPreserved", func(t *testing.T) {
		stored, err := parseUserAgentRegexp(`(Brightbot 1\.0)`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, ok := stored[`Brightbot 1\.0`]
		if !ok {
			t.Errorf("expected escaped value in set, got %v", stored)
		}
	})

	t.Run("EmptyGroup", func(t *testing.T) {
		stored, err := parseUserAgentRegexp(`()`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(stored) != 0 {
			t.Errorf("expected empty set, got %v", stored)
		}
	})

	t.Run("Malformed", func(t *testing.T) {
		_, err := parseUserAgentRegexp(`GPTBot`)
		if err == nil {
			t.Fatal("expected error for an expression without parentheses")
		}
	})
}

func TestAddUserAgents(t *testing.T) {
	t.Run("Append", func(t *testing.T) {
		regExpr, err := AddUserAgents(`(GPTBot)`, "SemrushBot/7~bl")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, compileErr := regexp.Compile(regExpr)
		if compileErr != nil {
			t.Fatalf("expected result to compile, got error: %v", compileErr)
		}
		stored, err := parseUserAgentRegexp(regExpr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(stored) != 2 {
			t.Fatalf("expected 2 user agents, got %v", stored)
		}
		for _, want := range []string{"GPTBot", "SemrushBot/7~bl"} {
			_, ok := stored[want]
			if !ok {
				t.Errorf("expected %q in set, got %v", want, stored)
			}
		}
	})

	t.Run("Variadic", func(t *testing.T) {
		regExpr, err := AddUserAgents(`(GPTBot)`, "SemrushBot", "Applebot")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		stored, err := parseUserAgentRegexp(regExpr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(stored) != 3 {
			t.Fatalf("expected 3 user agents, got %v", stored)
		}
		for _, want := range []string{"GPTBot", "SemrushBot", "Applebot"} {
			_, ok := stored[want]
			if !ok {
				t.Errorf("expected %q in set, got %v", want, stored)
			}
		}
	})

	t.Run("EmptyValue", func(t *testing.T) {
		_, err := AddUserAgents(`(GPTBot)`, "")
		if err == nil {
			t.Fatal("expected error for an empty user agent")
		}
	})

	t.Run("AlreadyPresent", func(t *testing.T) {
		regExpr, err := AddUserAgents(`(GPTBot|SemrushBot)`, "GPTBot")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if regExpr != `(GPTBot|SemrushBot)` {
			t.Errorf("expected the expression to stay unchanged, got %q", regExpr)
		}
	})

	t.Run("EmptyGroup", func(t *testing.T) {
		regExpr, err := AddUserAgents(`()`, "GPTBot")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if regExpr != `(GPTBot)` {
			t.Errorf("expected %q, got %q", `(GPTBot)`, regExpr)
		}
	})

	t.Run("Malformed", func(t *testing.T) {
		_, err := AddUserAgents(`GPTBot`, "SemrushBot")
		if err == nil {
			t.Fatal("expected error for an expression without parentheses")
		}
	})

	t.Run("TooLong", func(t *testing.T) {
		regExpr := `()`
		for i := 0; i < maxUserAgents; i++ {
			var err error
			regExpr, err = AddUserAgents(regExpr, "Bot"+strconv.Itoa(i))
			if err != nil {
				t.Fatalf("unexpected error at %d: %v", i, err)
			}
		}

		_, err := AddUserAgents(regExpr, "BotOverflow")
		if !errors.Is(err, ErrTooManyUserAgents) {
			t.Fatalf("expected error to wrap ErrTooManyUserAgents, got %v", err)
		}
	})
}
