package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFaviconHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(FaviconHandler)
	handler.ServeHTTP(rr, req)

	t.Run("StatusCode", func(t *testing.T) {
		if status := rr.Code; status != http.StatusNoContent {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusNoContent)
		}
	})

	t.Run("CacheControlHeader", func(t *testing.T) {
		expected := "public, max-age=86400"
		if val := rr.Header().Get("Cache-Control"); val != expected {
			t.Errorf("handler returned wrong Cache-Control header: got %q want %q",
				val, expected)
		}
	})

	t.Run("BodyContent", func(t *testing.T) {
		if body := rr.Body.String(); body != "" {
			t.Errorf("handler returned unexpected body: got %q want empty", body)
		}
	})
}
