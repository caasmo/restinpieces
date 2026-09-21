package s3

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestS3_GetObject(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
			}
			if r.URL.Path != "/test-bucket/mydb/0/12-15.ltx" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/test-bucket/mydb/0/12-15.ltx")
			}

			w.Header().Set("ETag", `"etag-1"`)
			w.Header().Set("Content-Length", "9")
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("ltx-bytes"))
		})

		resp, err := client.GetObject(context.Background(), "mydb/0/12-15.ltx")
		if err != nil {
			t.Fatalf("GetObject() failed: %v", err)
		}
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Errorf("failed to close body: %v", closeErr)
			}
		}()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if string(body) != "ltx-bytes" {
			t.Errorf("body = %q, want %q", body, "ltx-bytes")
		}
		if resp.ETag != `"etag-1"` {
			t.Errorf("etag = %q, want %q", resp.ETag, `"etag-1"`)
		}
		if resp.ContentLength != 9 {
			t.Errorf("content length = %d, want %d", resp.ContentLength, 9)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`))
		})

		_, err := client.GetObject(context.Background(), "mydb/0/missing.ltx")

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
