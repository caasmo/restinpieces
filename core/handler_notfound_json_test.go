package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotFoundJSONHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rr := httptest.NewRecorder()

	NotFoundJSONHandler(rr, req)

	t.Run("StatusCode", func(t *testing.T) {
		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusNotFound)
		}
	})

	t.Run("ContentType", func(t *testing.T) {
		expected := HeadersJson["Content-Type"]
		if val := rr.Header().Get("Content-Type"); val != expected {
			t.Errorf("handler returned wrong Content-Type: got %q want %q",
				val, expected)
		}
	})

	t.Run("BodyContent", func(t *testing.T) {
		var resp JsonBasic
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response body: %v", err)
		}

		if resp.Status != http.StatusNotFound {
			t.Errorf("unexpected status in body: got %v want %v", resp.Status, http.StatusNotFound)
		}
		if resp.Code != CodeErrorNotFound {
			t.Errorf("unexpected code in body: got %q want %q", resp.Code, CodeErrorNotFound)
		}
		if resp.Message == "" {
			t.Error("expected non-empty message in body")
		}
	})
}
