package config

import (
	"fmt"
	"net/url"
)

// S3 holds the connection settings for the S3 client in the s3 package.
// The framework does not run the client; an application or a companion
// program reads this section and builds the client from it. An empty
// Endpoint leaves the section off and the other keys are ignored. A set
// Endpoint requires Region, Bucket, AccessKey and SecretKey.
type S3 struct {
	// Endpoint is the storage URL, for example "https://s3.example.com".
	// It must use http:// or https://. Empty string leaves the section off.
	Endpoint string `toml:"endpoint" comment:"Storage URL with the scheme (e.g. 'https://s3.example.com'). Empty leaves the section off."`

	// Region is the provider's region, for example "us-east-1" or "auto"
	// for Cloudflare R2.
	Region string `toml:"region" comment:"Provider region (e.g. 'us-east-1', 'auto' for Cloudflare R2)"`

	// Bucket is the bucket that holds the objects.
	Bucket string `toml:"bucket" comment:"Bucket that holds the objects"`

	// AccessKey is the S3 access key. Store it with ripc set, which keeps
	// the configuration encrypted in the database.
	AccessKey string `toml:"access_key" comment:"Access key (set via ripc set)"`

	// SecretKey is the S3 secret key. Store it with ripc set, which keeps
	// the configuration encrypted in the database.
	SecretKey string `toml:"secret_key" comment:"Secret key (set via ripc set)"`

	// UsePathStyle puts the bucket in the URL path instead of the host
	// name. False uses virtual-hosted addressing, which Amazon S3 uses by
	// default; Cloudflare R2 and MinIO require true.
	UsePathStyle bool `toml:"use_path_style" comment:"Use path-style addressing (true for Cloudflare R2 and MinIO)"`

	// RequireContentLength tells the program that the service needs a
	// length on every put. Some services, Cloudflare R2 for example, reject
	// the chunked upload that an unknown length produces; set it to true
	// for them.
	RequireContentLength bool `toml:"require_content_length" comment:"Require a content length on every put (true for Cloudflare R2)"`
}

// validateS3 checks the S3 configuration section. An empty endpoint means
// the section is not configured and is ignored. A set endpoint must be an
// http:// or https:// URL, and region, bucket, access key and secret key
// are required. The scheme is required even though the s3 package accepts
// a schemeless endpoint; the stored config always keeps the scheme.
func validateS3(s3 *S3) error {
	if s3.Endpoint == "" {
		return nil
	}

	parsed, err := url.Parse(s3.Endpoint)
	if err != nil {
		return fmt.Errorf("s3.endpoint %q is not a valid URL: %w", s3.Endpoint, err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("s3.endpoint %q must start with http:// or https://", s3.Endpoint)
	}
	if parsed.Host == "" {
		return fmt.Errorf("s3.endpoint %q must include a host", s3.Endpoint)
	}
	if s3.Region == "" {
		return fmt.Errorf("s3.region cannot be empty when s3.endpoint is set")
	}
	if s3.Bucket == "" {
		return fmt.Errorf("s3.bucket cannot be empty when s3.endpoint is set")
	}
	if s3.AccessKey == "" {
		return fmt.Errorf("s3.access_key cannot be empty when s3.endpoint is set")
	}
	if s3.SecretKey == "" {
		return fmt.Errorf("s3.secret_key cannot be empty when s3.endpoint is set")
	}

	return nil
}
