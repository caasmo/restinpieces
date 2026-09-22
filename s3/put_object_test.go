package s3

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestS3_PutObject(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		var gotMethod string
		var gotPath string
		var gotBody []byte
		var gotAuth string
		var gotPayloadHash string
		var gotContentLength int64

		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			gotPayloadHash = r.Header.Get("x-amz-content-sha256")
			gotContentLength = r.ContentLength

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("failed to read request body: %v", err)
			}
			gotBody = body

			w.WriteHeader(http.StatusOK)
		})

		// LimitReader keeps the HTTP client from detecting a length on
		// its own, so the size passed to PutObject is what the request sends.
		body := io.LimitReader(strings.NewReader("ltx-bytes"), int64(len("ltx-bytes")))
		err := client.PutObject(context.Background(), "mydb/0/12-15.ltx", body, int64(len("ltx-bytes")))
		if err != nil {
			t.Fatalf("PutObject() failed: %v", err)
		}

		if gotMethod != http.MethodPut {
			t.Errorf("method = %q, want %q", gotMethod, http.MethodPut)
		}
		if gotPath != "/test-bucket/mydb/0/12-15.ltx" {
			t.Errorf("path = %q, want %q", gotPath, "/test-bucket/mydb/0/12-15.ltx")
		}
		if string(gotBody) != "ltx-bytes" {
			t.Errorf("body = %q, want %q", gotBody, "ltx-bytes")
		}
		if gotContentLength != int64(len("ltx-bytes")) {
			t.Errorf("content length = %d, want %d", gotContentLength, len("ltx-bytes"))
		}
		if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=test-access-key/") {
			t.Errorf("authorization = %q, want the AWS4-HMAC-SHA256 credential prefix", gotAuth)
		}
		if gotPayloadHash != "UNSIGNED-PAYLOAD" {
			t.Errorf("x-amz-content-sha256 = %q, want %q", gotPayloadHash, "UNSIGNED-PAYLOAD")
		}
	})

	t.Run("UnknownSize", func(t *testing.T) {
		var gotBody []byte
		var gotContentLength int64
		var gotTransferEncoding []string

		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotContentLength = r.ContentLength
			gotTransferEncoding = r.TransferEncoding

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("failed to read request body: %v", err)
			}
			gotBody = body

			w.WriteHeader(http.StatusOK)
		})

		// A negative size must produce a chunked request; LimitReader keeps
		// the HTTP client from detecting a length on its own.
		body := io.LimitReader(strings.NewReader("ltx-bytes"), int64(len("ltx-bytes")))
		err := client.PutObject(context.Background(), "mydb/0/12-15.ltx", body, -1)
		if err != nil {
			t.Fatalf("PutObject() failed: %v", err)
		}

		if string(gotBody) != "ltx-bytes" {
			t.Errorf("body = %q, want %q", gotBody, "ltx-bytes")
		}
		if gotContentLength != -1 {
			t.Errorf("content length = %d, want -1", gotContentLength)
		}
		if len(gotTransferEncoding) != 1 || gotTransferEncoding[0] != "chunked" {
			t.Errorf("transfer encoding = %v, want [chunked]", gotTransferEncoding)
		}
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`<Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`))
		})

		body := io.LimitReader(strings.NewReader("ltx-bytes"), int64(len("ltx-bytes")))
		err := client.PutObject(context.Background(), "mydb/0/12-15.ltx", body, int64(len("ltx-bytes")))

		respErr, ok := err.(*ResponseError)
		if !ok {
			t.Fatalf("expected *ResponseError, got %T: %v", err, err)
		}
		if respErr.Status != http.StatusForbidden {
			t.Errorf("status = %d, want %d", respErr.Status, http.StatusForbidden)
		}
		if respErr.Code != "AccessDenied" {
			t.Errorf("code = %q, want %q", respErr.Code, "AccessDenied")
		}
	})
}
