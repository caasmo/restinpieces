package ssr

import (
	"bytes"
	"math"
	"testing"
)

func TestMarshal(t *testing.T) {
	t.Run("SuccessWithNonce", func(t *testing.T) {
		html := []byte(`<html><head><title>Test</title></head><body>Hello</body></html>`)
		data := map[string]any{"foo": "bar", "count": 42}
		nonce := "random-nonce"

		got, err := Marshal(html, data, nonce)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		expectedScript := `<script nonce="random-nonce">window.__INITIAL_DATA__={"count":42,"foo":"bar"};</script></head>`
		if !bytes.Contains(got, []byte(expectedScript)) {
			t.Errorf("Expected script not found in output.\nGot: %s", string(got))
		}

		if !bytes.Contains(got, []byte("<title>Test</title>")) {
			t.Error("Original head content lost")
		}

		if !bytes.Contains(got, []byte("<body>Hello</body>")) {
			t.Error("Body content lost")
		}
	})

	t.Run("SuccessNoNonce", func(t *testing.T) {
		html := []byte(`<html><head></head><body>Hello</body></html>`)
		data := map[string]any{"foo": "bar"}
		nonce := ""

		got, err := Marshal(html, data, nonce)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		expectedScript := `<script>window.__INITIAL_DATA__={"foo":"bar"};</script></head>`
		if !bytes.Contains(got, []byte(expectedScript)) {
			t.Errorf("Expected script not found in output.\nGot: %s", string(got))
		}
	})

	t.Run("ErrorNoHeadTag", func(t *testing.T) {
		html := []byte(`<html><body>No head tag here</body></html>`)
		data := map[string]any{"foo": "bar"}

		_, err := Marshal(html, data, "")
		if err == nil {
			t.Fatal("Expected error due to missing </head> tag, but got nil")
		}

		expectedErr := "ssr.Marshal: </head> tag not found in payload"
		if err.Error() != expectedErr {
			t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("ErrorJSONMarshalFailure", func(t *testing.T) {
		html := []byte(`<html><head></head></html>`)
		// Unsupported type for JSON marshal
		data := map[string]any{"foo": math.Inf(1)}

		_, err := Marshal(html, data, "")
		if err == nil {
			t.Fatal("Expected error due to JSON marshal failure, but got nil")
		}

		// The error should mention encoding failure
		if !bytes.Contains([]byte(err.Error()), []byte("ssr.Marshal: failed to encode data")) {
			t.Errorf("Expected error message to contain encoding failure, got: %v", err)
		}
	})
}
