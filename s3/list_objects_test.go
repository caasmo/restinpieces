package s3

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestS3_ListObjects(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		// an empty object key leaves the trailing slash on the bucket URL
		if r.URL.Path != "/test-bucket/" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/test-bucket/")
		}

		query := r.URL.Query()
		if query.Get("list-type") != "2" {
			t.Errorf("list-type = %q, want %q", query.Get("list-type"), "2")
		}
		if query.Get("prefix") != "mydb/0/" {
			t.Errorf("prefix = %q, want %q", query.Get("prefix"), "mydb/0/")
		}
		if query.Get("continuation-token") != "token-2" {
			t.Errorf("continuation-token = %q, want %q", query.Get("continuation-token"), "token-2")
		}

		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
  <Name>test-bucket</Name>
  <Prefix>mydb/0/</Prefix>
  <KeyCount>2</KeyCount>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken>token-3</NextContinuationToken>
  <Contents>
    <Key>mydb/0/12-15.ltx</Key>
    <LastModified>2026-01-02T15:04:05.000Z</LastModified>
    <ETag>"etag-1"</ETag>
    <Size>1024</Size>
  </Contents>
  <Contents>
    <Key>mydb/0/16-19.ltx</Key>
    <LastModified>2026-01-02T15:05:05.000Z</LastModified>
    <ETag>"etag-2"</ETag>
    <Size>2048</Size>
  </Contents>
</ListBucketResult>`))
	})

	list, err := client.ListObjects(context.Background(), ListParams{
		Prefix:            "mydb/0/",
		ContinuationToken: "token-2",
	})
	if err != nil {
		t.Fatalf("ListObjects() failed: %v", err)
	}

	if len(list.Contents) != 2 {
		t.Fatalf("contents length = %d, want 2", len(list.Contents))
	}
	if list.Contents[0].Key != "mydb/0/12-15.ltx" {
		t.Errorf("key = %q, want %q", list.Contents[0].Key, "mydb/0/12-15.ltx")
	}
	if list.Contents[0].Size != 1024 {
		t.Errorf("size = %d, want %d", list.Contents[0].Size, 1024)
	}

	wantTime := time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
	if !list.Contents[0].LastModified.Equal(wantTime) {
		t.Errorf("last modified = %s, want %s", list.Contents[0].LastModified, wantTime)
	}
	if !list.IsTruncated {
		t.Error("is truncated = false, want true")
	}
	if list.NextContinuationToken != "token-3" {
		t.Errorf("next continuation token = %q, want %q", list.NextContinuationToken, "token-3")
	}
}

func TestListParams_Encode(t *testing.T) {
	params := ListParams{
		Prefix:            "mydb/0/",
		ContinuationToken: "token-2",
		Delimiter:         "/",
		MaxKeys:           1000,
		FetchOwner:        true,
	}

	expected := "continuation-token=token-2&delimiter=%2F&fetch-owner=true&list-type=2&max-keys=1000&prefix=mydb%2F0%2F"

	got := params.Encode()
	if got != expected {
		t.Errorf("Encode() = %q, want %q", got, expected)
	}
}
