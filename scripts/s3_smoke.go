//go:build ignore

// Command s3_smoke exercises the s3 client against a real S3-compatible
// bucket: it uploads a small object, reads it back, heads it, lists it
// and deletes it, printing each step to stdout.
//
// The bucket settings come from the environment:
//
//	S3_TEST_ENDPOINT    e.g. https://<account-id>.r2.cloudflarestorage.com
//	S3_TEST_REGION      e.g. auto for R2, us-west-002 for Backblaze
//	S3_TEST_BUCKET
//	S3_TEST_ACCESS_KEY
//	S3_TEST_SECRET_KEY
//	S3_TEST_PATH_STYLE  optional; set to "false" for virtual-hosted addressing
//	                    (path-style is the default and what R2/MinIO need)
//
// Usage (from repo root):
//
//	go run scripts/s3_smoke.go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/caasmo/restinpieces/s3"
)

func main() {
	os.Exit(run())
}

func run() int {
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	region := os.Getenv("S3_TEST_REGION")
	bucket := os.Getenv("S3_TEST_BUCKET")
	accessKey := os.Getenv("S3_TEST_ACCESS_KEY")
	secretKey := os.Getenv("S3_TEST_SECRET_KEY")

	missing := []string{}
	if endpoint == "" {
		missing = append(missing, "S3_TEST_ENDPOINT")
	}
	if region == "" {
		missing = append(missing, "S3_TEST_REGION")
	}
	if bucket == "" {
		missing = append(missing, "S3_TEST_BUCKET")
	}
	if accessKey == "" {
		missing = append(missing, "S3_TEST_ACCESS_KEY")
	}
	if secretKey == "" {
		missing = append(missing, "S3_TEST_SECRET_KEY")
	}
	if len(missing) > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "missing env vars: %v\n", missing)
		return 2
	}

	// Cloudflare R2 and most MinIO setups need path-style requests.
	// Set S3_TEST_PATH_STYLE=false to use virtual-hosted addressing.
	usePathStyle := os.Getenv("S3_TEST_PATH_STYLE") != "false"

	client := &s3.S3{
		Endpoint:     endpoint,
		Region:       region,
		Bucket:       bucket,
		AccessKey:    accessKey,
		SecretKey:    secretKey,
		UsePathStyle: usePathStyle,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := fmt.Sprintf("smoke/%d.ltx", time.Now().UnixNano())
	payload := []byte("restinpieces s3 smoke test")

	_, _ = fmt.Printf("PUT %s ... ", key)
	err := client.PutObject(ctx, key, payload)
	if err != nil {
		_, _ = fmt.Printf("FAIL: %v\n", err)
		return 1
	}
	_, _ = fmt.Printf("ok\n")

	_, _ = fmt.Printf("HEAD %s ... ", key)
	info, err := client.HeadObject(ctx, key)
	if err != nil {
		_, _ = fmt.Printf("FAIL: %v\n", err)
		return 1
	}
	if info.ContentLength != int64(len(payload)) {
		_, _ = fmt.Printf("FAIL: content length = %d, want %d\n", info.ContentLength, len(payload))
		return 1
	}
	_, _ = fmt.Printf("ok (size %d)\n", info.ContentLength)

	_, _ = fmt.Printf("GET %s ... ", key)
	resp, err := client.GetObject(ctx, key)
	if err != nil {
		_, _ = fmt.Printf("FAIL: %v\n", err)
		return 1
	}
	body, err := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err != nil {
		_, _ = fmt.Printf("FAIL reading body: %v\n", err)
		return 1
	}
	if closeErr != nil {
		_, _ = fmt.Printf("FAIL closing body: %v\n", closeErr)
		return 1
	}
	if string(body) != string(payload) {
		_, _ = fmt.Printf("FAIL: body = %q, want %q\n", body, payload)
		return 1
	}
	_, _ = fmt.Printf("ok (%d bytes)\n", len(body))

	_, _ = fmt.Printf("LIST %s ... ", key)
	list, err := client.ListObjects(ctx, s3.ListParams{Prefix: key})
	if err != nil {
		_, _ = fmt.Printf("FAIL: %v\n", err)
		return 1
	}
	found := false
	for _, obj := range list.Contents {
		if obj.Key == key {
			found = true
			break
		}
	}
	if !found {
		_, _ = fmt.Printf("FAIL: key %q not found in listing\n", key)
		return 1
	}
	_, _ = fmt.Printf("ok\n")

	_, _ = fmt.Printf("DELETE %s ... ", key)
	err = client.DeleteObject(ctx, key)
	if err != nil {
		_, _ = fmt.Printf("FAIL: %v\n", err)
		return 1
	}
	_, _ = fmt.Printf("ok\n")

	_, _ = fmt.Printf("smoke passed\n")
	return 0
}
