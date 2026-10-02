package s3

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// testBucket is the bucket the operation tests address.
const testBucket = "test-bucket"

// newTestClient returns a client pointed at a test server that answers
// every request with handler. The server is closed when the test ends.
func newTestClient(t *testing.T, handler http.HandlerFunc) *S3 {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return &S3{
		Endpoint:     server.URL,
		Region:       "test-region",
		AccessKey:    "test-access-key",
		SecretKey:    "test-secret-key",
		UsePathStyle: true,
	}
}
