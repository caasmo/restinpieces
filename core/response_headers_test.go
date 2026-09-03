package core

import (
	"net/http/httptest"
	"testing"
)

func TestSetHeaders(t *testing.T) {
	t.Run("single map", func(t *testing.T) {
		rr := httptest.NewRecorder()
		headers := map[string]string{
			"X-Test": "Value",
		}
		SetHeaders(rr, headers)
		if got := rr.Header().Get("X-Test"); got != "Value" {
			t.Errorf("expected Value, got %q", got)
		}
	})

	t.Run("multiple maps with merge", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h1 := map[string]string{"A": "1", "B": "2"}
		h2 := map[string]string{"B": "override", "C": "3"}
		SetHeaders(rr, h1, h2)

		if rr.Header().Get("A") != "1" {
			t.Errorf("A: expected 1")
		}
		if rr.Header().Get("B") != "override" {
			t.Errorf("B: expected override, got %q", rr.Header().Get("B"))
		}
		if rr.Header().Get("C") != "3" {
			t.Errorf("C: expected 3")
		}
	})

	t.Run("canonicalization", func(t *testing.T) {
		rr := httptest.NewRecorder()
		headers := map[string]string{
			"content-type": "test/plain",
		}
		SetHeaders(rr, headers)
		// http.Header.Get uses canonical keys internally
		if got := rr.Header().Get("Content-Type"); got != "test/plain" {
			t.Errorf("expected test/plain, got %q", got)
		}
	})
}

func TestResponseHeaderValues(t *testing.T) {
	// Verify critical security headers in our predefined maps

	t.Run("HeadersJson", func(t *testing.T) {
		h := HeadersJson
		checks := map[string]string{
			"Content-Type":            "application/json; charset=utf-8",
			"X-Content-Type-Options":  "nosniff",
			"Cache-Control":           "no-store",
			"X-Frame-Options":         "DENY",
			"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		}
		for k, expected := range checks {
			if h[k] != expected {
				t.Errorf("%s: expected %q, got %q", k, expected, h[k])
			}
		}
	})

	t.Run("HeadersHtml", func(t *testing.T) {
		h := HeadersHtml
		checks := map[string]string{
			"Cache-Control":           "private, no-cache",
			"X-Content-Type-Options":  "nosniff",
			"Referrer-Policy":         "strict-origin-when-cross-origin",
			"Content-Security-Policy": StrictCSP,
		}
		for k, expected := range checks {
			if h[k] != expected {
				t.Errorf("%s: expected %q, got %q", k, expected, h[k])
			}
		}
	})

	t.Run("HeadersMaintenancePage", func(t *testing.T) {
		if HeadersMaintenancePage["Cache-Control"] != "no-store" {
			t.Errorf("expected no-store, got %q", HeadersMaintenancePage["Cache-Control"])
		}
		if HeadersMaintenancePage["Retry-After"] != "600" {
			t.Errorf("expected 600, got %q", HeadersMaintenancePage["Retry-After"])
		}
	})
}
