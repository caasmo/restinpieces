//go:build ignore

// Command s3_probe exercises the s3 client against a real S3-compatible
// bucket: it uploads a small object, reads it back, heads it, lists it
// and deletes it, printing each step to stdout.
//
// It reads what the app reads: endpoint, region and keys come from the
// latest config in the application database, the bucket comes from the
// command line. Runs in the deploy directory: the database is
// data/app.db and the age key is age.key. Prod only.
//
// Usage (from repo root):
//
//	go run scripts/s3_probe.go <bucket>
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/caasmo/restinpieces"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db/databasesql"
	"github.com/caasmo/restinpieces/s3"
	"github.com/pelletier/go-toml/v2"
)

const (
	dbPath     = "data/app.db"
	ageKeyPath = "age.key"

	// Prefix and suffix for the throwaway probe object. The key also
	// carries the current time so concurrent runs never collide.
	objectPrefix = "smoke/"
	objectSuffix = ".ltx"

	// requestTimeout caps the whole probe. A stuck endpoint must fail
	// loudly, not hang the shell.
	requestTimeout = 30 * time.Second
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <bucket>\n", os.Args[0])
		os.Exit(1)
	}
	bucket := os.Args[1]

	pool, err := restinpieces.NewModerncPool(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db %q: %v\n", dbPath, err)
		os.Exit(1)
	}
	defer func() {
		_ = pool.Close()
	}()

	dbInstance, err := databasesql.New(pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init db: %v\n", err)
		os.Exit(1)
	}

	secureStore, err := config.NewSecureStoreAge(dbInstance, ageKeyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init config store: %v\n", err)
		os.Exit(1)
	}

	encrypted, _, err := secureStore.Get(config.ScopeApplication, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load latest config: %v\n", err)
		os.Exit(1)
	}

	cfg := config.NewDefaultConfig()
	if err := toml.Unmarshal(encrypted, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "parse config: %v\n", err)
		os.Exit(1)
	}
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}

	client := &s3.S3{
		Endpoint:             cfg.S3.Endpoint,
		Region:               cfg.S3.Region,
		AccessKey:            cfg.S3.AccessKey,
		SecretKey:            cfg.S3.SecretKey,
		UsePathStyle:         cfg.S3.UsePathStyle,
		RequireContentLength: cfg.S3.RequireContentLength,
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	fmt.Printf("endpoint %s bucket %s\n", cfg.S3.Endpoint, bucket)

	key := fmt.Sprintf("%s%d%s", objectPrefix, time.Now().UnixNano(), objectSuffix)
	payload := []byte("restinpieces s3 probe test")

	fmt.Printf("PUT %s ... ", key)
	if err := client.PutObject(ctx, bucket, key, bytes.NewReader(payload), int64(len(payload))); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("ok\n")

	fmt.Printf("HEAD %s ... ", key)
	info, err := client.HeadObject(ctx, bucket, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	if info.ContentLength != int64(len(payload)) {
		fmt.Fprintf(os.Stderr, "FAIL: content length = %d, want %d\n", info.ContentLength, len(payload))
		os.Exit(1)
	}
	fmt.Printf("ok (size %d)\n", info.ContentLength)

	fmt.Printf("GET %s ... ", key)
	resp, err := client.GetObject(ctx, bucket, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close response body: %v\n", err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL reading body: %v\n", err)
		os.Exit(1)
	}
	if string(body) != string(payload) {
		fmt.Fprintf(os.Stderr, "FAIL: body = %q, want %q\n", body, payload)
		os.Exit(1)
	}
	fmt.Printf("ok (%d bytes)\n", len(body))

	fmt.Printf("LIST %s ... ", key)
	list, err := client.ListObjects(ctx, bucket, s3.ListParams{Prefix: key})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	found := false
	for _, obj := range list.Contents {
		if obj.Key == key {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "FAIL: key %q not found in listing\n", key)
		os.Exit(1)
	}
	fmt.Printf("ok\n")

	fmt.Printf("DELETE %s ... ", key)
	if err := client.DeleteObject(ctx, bucket, key); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("ok\n")

	fmt.Printf("probe passed\n")
}
