package s3

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestS3_HeadObject(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodHead {
				t.Errorf("method = %q, want %q", r.Method, http.MethodHead)
			}
			if r.URL.Path != "/test-bucket/mydb/0/12-15.ltx" {
				t.Errorf("path = %q, want %q", r.URL.Path, "/test-bucket/mydb/0/12-15.ltx")
			}

			w.Header().Set("ETag", `"etag-1"`)
			w.Header().Set("Content-Length", "9")
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Last-Modified", "Fri, 02 Jan 2026 15:04:05 GMT")
			w.Header().Set("x-amz-meta-origin", "test")
			w.WriteHeader(http.StatusOK)
		})

		info, err := client.HeadObject(context.Background(), "mydb/0/12-15.ltx")
		if err != nil {
			t.Fatalf("HeadObject() failed: %v", err)
		}

		if info.ETag != `"etag-1"` {
			t.Errorf("etag = %q, want %q", info.ETag, `"etag-1"`)
		}
		if info.ContentLength != 9 {
			t.Errorf("content length = %d, want %d", info.ContentLength, 9)
		}

		wantTime := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
		if !info.LastModified.Equal(wantTime) {
			t.Errorf("last modified = %s, want %s", info.LastModified, wantTime)
		}
		if info.Metadata["origin"] != "test" {
			t.Errorf("metadata origin = %q, want %q", info.Metadata["origin"], "test")
		}
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		// A HEAD response carries no body, so the server sends none and
		// the client can report only the status code.
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := client.HeadObject(context.Background(), "mydb/0/missing.ltx")

		respErr, ok := err.(*ResponseError)
		if !ok {
			t.Fatalf("expected *ResponseError, got %T: %v", err, err)
		}
		if respErr.Status != http.StatusNotFound {
			t.Errorf("status = %d, want %d", respErr.Status, http.StatusNotFound)
		}
		if respErr.Code != "" {
			t.Errorf("code = %q, want empty (HEAD responses carry no body)", respErr.Code)
		}
	})
}
