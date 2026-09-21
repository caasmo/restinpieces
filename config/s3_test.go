package config

import (
	"testing"
)

func TestValidateS3(t *testing.T) {
	t.Parallel()
	validCases := []S3{
		{},
		{Endpoint: "https://s3.example.com", Region: "us-east-1", Bucket: "my-bucket", AccessKey: "ak", SecretKey: "sk"},
		{Endpoint: "http://127.0.0.1:9000", Region: "us-east-1", Bucket: "my-bucket", AccessKey: "ak", SecretKey: "sk"},
	}
	for _, cfg := range validCases {
		if err := validateS3(&cfg); err != nil {
			t.Errorf("validateS3(%+v) failed: %v", cfg, err)
		}
	}

	invalidCases := []S3{
		{Endpoint: "s3.example.com", Region: "us-east-1", Bucket: "my-bucket", AccessKey: "ak", SecretKey: "sk"},
		{Endpoint: "ftp://s3.example.com", Region: "us-east-1", Bucket: "my-bucket", AccessKey: "ak", SecretKey: "sk"},
		{Endpoint: "https://", Region: "us-east-1", Bucket: "my-bucket", AccessKey: "ak", SecretKey: "sk"},
		{Endpoint: "https://s3.example.com"},
		{Endpoint: "https://s3.example.com", Bucket: "my-bucket", AccessKey: "ak", SecretKey: "sk"},
		{Endpoint: "https://s3.example.com", Region: "us-east-1", AccessKey: "ak", SecretKey: "sk"},
		{Endpoint: "https://s3.example.com", Region: "us-east-1", Bucket: "my-bucket", SecretKey: "sk"},
		{Endpoint: "https://s3.example.com", Region: "us-east-1", Bucket: "my-bucket", AccessKey: "ak"},
	}
	for _, cfg := range invalidCases {
		if err := validateS3(&cfg); err == nil {
			t.Errorf("validateS3(%+v) expected error, got nil", cfg)
		}
	}
}
