package s3

import (
	"net/http"
	"strings"
	"testing"
)

func TestS3_URL(t *testing.T) {
	path := "/test_key/a/b c@d?a=@1&b=!2#@a b c"
	expectedPath := "/test_key/a/b%20c%40d?a=@1&b=!2#@a b c"

	testCases := []struct {
		name     string
		client   *S3
		expected string
	}{
		{
			name: "no scheme",
			client: &S3{
				Region:    "test_region",
				Bucket:    "test_bucket",
				Endpoint:  "example.com/",
				AccessKey: "123",
				SecretKey: "abc",
			},
			expected: "https://test_bucket.example.com" + expectedPath,
		},
		{
			name: "https scheme",
			client: &S3{
				Region:    "test_region",
				Bucket:    "test_bucket",
				Endpoint:  "https://example.com/",
				AccessKey: "123",
				SecretKey: "abc",
			},
			expected: "https://test_bucket.example.com" + expectedPath,
		},
		{
			name: "http scheme",
			client: &S3{
				Region:    "test_region",
				Bucket:    "test_bucket",
				Endpoint:  "http://example.com/",
				AccessKey: "123",
				SecretKey: "abc",
			},
			expected: "http://test_bucket.example.com" + expectedPath,
		},
		{
			name: "path style without scheme",
			client: &S3{
				Region:       "test_region",
				Bucket:       "test_bucket",
				Endpoint:     "example.com/",
				AccessKey:    "123",
				SecretKey:    "abc",
				UsePathStyle: true,
			},
			expected: "https://example.com/test_bucket" + expectedPath,
		},
		{
			name: "path style with scheme",
			client: &S3{
				Region:       "test_region",
				Bucket:       "test_bucket",
				Endpoint:     "http://example.com/",
				AccessKey:    "123",
				SecretKey:    "abc",
				UsePathStyle: true,
			},
			expected: "http://example.com/test_bucket" + expectedPath,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.client.URL(path)
			if result != tc.expected {
				t.Fatalf("expected URL\n%s\ngot\n%s", tc.expected, result)
			}
		})
	}
}

func TestS3_Sign(t *testing.T) {
	testCases := []struct {
		name     string
		client   *S3
		path     string
		reqFunc  func(req *http.Request)
		expected map[string]string
	}{
		{
			name: "minimal",
			client: &S3{
				Region:    "test_region",
				Bucket:    "test_bucket",
				Endpoint:  "https://example.com/",
				AccessKey: "123",
				SecretKey: "abc",
			},
			path: "/test",
			reqFunc: func(req *http.Request) {
				req.Header.Set("x-amz-date", "20250102T150405Z")
			},
			expected: map[string]string{
				"Authorization":        "AWS4-HMAC-SHA256 Credential=123/20250102/test_region/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=ea093662bc1deef08dfb4ac35453dfaad5ea89edf102e9dd3b7156c9a27e4c1f",
				"Host":                 "test_bucket.example.com",
				"Accept-Encoding":      "identity",
				"X-Amz-Content-Sha256": "UNSIGNED-PAYLOAD",
				"X-Amz-Date":           "20250102T150405Z",
			},
		},
		{
			name: "different access and secret keys",
			client: &S3{
				Region:    "test_region",
				Bucket:    "test_bucket",
				Endpoint:  "https://example.com/",
				AccessKey: "456",
				SecretKey: "def",
			},
			path: "/test",
			reqFunc: func(req *http.Request) {
				req.Header.Set("x-amz-date", "20250102T150405Z")
			},
			expected: map[string]string{
				"Authorization":        "AWS4-HMAC-SHA256 Credential=456/20250102/test_region/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=17510fa1f724403dd0a563b61c9b31d1d718f877fcbd75455620d17a8afce5fb",
				"Host":                 "test_bucket.example.com",
				"Accept-Encoding":      "identity",
				"X-Amz-Content-Sha256": "UNSIGNED-PAYLOAD",
				"X-Amz-Date":           "20250102T150405Z",
			},
		},
		{
			name: "special characters in the path",
			client: &S3{
				Region:    "test_region",
				Bucket:    "test_bucket",
				Endpoint:  "https://example.com/",
				AccessKey: "456",
				SecretKey: "def",
			},
			path: "/ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 -_.~!@&*():=$()?a=1&@b=@2#@a b c",
			reqFunc: func(req *http.Request) {
				req.Header.Set("x-amz-date", "20250102T150405Z")
			},
			expected: map[string]string{
				"Authorization":        "AWS4-HMAC-SHA256 Credential=456/20250102/test_region/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=9458a033554f52913801b3de16f54409b36ed25c6da3aed14e64439500e2c5e1",
				"Host":                 "test_bucket.example.com",
				"Accept-Encoding":      "identity",
				"X-Amz-Content-Sha256": "UNSIGNED-PAYLOAD",
				"X-Amz-Date":           "20250102T150405Z",
			},
		},
		{
			name: "extra headers",
			client: &S3{
				Region:    "test_region",
				Bucket:    "test_bucket",
				Endpoint:  "https://example.com/",
				AccessKey: "123",
				SecretKey: "abc",
			},
			path: "/test",
			reqFunc: func(req *http.Request) {
				req.Header.Set("x-amz-date", "20250102T150405Z")
				req.Header.Set("x-amz-content-sha256", "test_sha256")
				req.Header.Set("x-amz-example", "123")
				req.Header.Set("x-amz-meta-a", "456")
				req.Header.Set("content-type", "image/png")
				req.Header.Set("accept-encoding", "custom")
				req.Header.Set("x-test", "789")
			},
			expected: map[string]string{
				"authorization":        "AWS4-HMAC-SHA256 Credential=123/20250102/test_region/s3/aws4_request, SignedHeaders=content-type;host;x-amz-content-sha256;x-amz-date;x-amz-example;x-amz-meta-a, Signature=86dccbcd012c33073dc99e9d0a9e0b717a4d8c11c37848cfa9a4a02716bc0db3",
				"host":                 "test_bucket.example.com",
				"accept-encoding":      "custom",
				"x-amz-date":           "20250102T150405Z",
				"x-amz-content-sha256": "test_sha256",
				"x-amz-example":        "123",
				"x-amz-meta-a":         "456",
				"x-test":               "789",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.client.URL(tc.path), strings.NewReader("test_request"))
			if err != nil {
				t.Fatalf("failed to build request: %v", err)
			}
			tc.reqFunc(req)

			tc.client.sign(req)

			for name, want := range tc.expected {
				got := req.Header.Get(name)
				if got != want {
					t.Errorf("header %s = %q, want %q", name, got, want)
				}
			}
		})
	}
}
