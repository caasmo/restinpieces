package s3

import (
	"context"
	"net/http"
	"testing"
)

func TestS3_DeleteObject(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		var gotMethod string
		var gotPath string

		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		})

		err := client.DeleteObject(context.Background(), "mydb/0/12-15.ltx")
		if err != nil {
			t.Fatalf("DeleteObject() failed: %v", err)
		}

		if gotMethod != http.MethodDelete {
			t.Errorf("method = %q, want %q", gotMethod, http.MethodDelete)
		}
		if gotPath != "/test-bucket/mydb/0/12-15.ltx" {
			t.Errorf("path = %q, want %q", gotPath, "/test-bucket/mydb/0/12-15.ltx")
		}
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`))
		})

		err := client.DeleteObject(context.Background(), "mydb/0/missing.ltx")

		respErr, ok := err.(*ResponseError)
		if !ok {
			t.Fatalf("expected *ResponseError, got %T: %v", err, err)
		}
		if respErr.Status != http.StatusNotFound {
			t.Errorf("status = %d, want %d", respErr.Status, http.StatusNotFound)
		}
		if respErr.Code != "NoSuchKey" {
			t.Errorf("code = %q, want %q", respErr.Code, "NoSuchKey")
		}
	})
}
