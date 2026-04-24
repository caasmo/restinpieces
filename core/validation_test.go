package core

import (
	"net/http/httptest"
	"testing"
)

func TestContentTypeValidation(t *testing.T) {
	const (
		contentTypeJSON            = "application/json"
		contentTypeJSONWithCharset = "application/json; charset=utf-8"
		contentTypeJSONWithParams  = "application/json; charset=utf-8; version=1"
		contentTypePlainText       = "text/plain"
	)

	testCases := []struct {
		name        string
		contentType string
		allowedType string
		wantError   jsonResponse
	}{
		{
			name:        "no content-type header",
			contentType: "",
			allowedType: contentTypeJSON,
			wantError:   errorInvalidContentType,
		},
		{
			name:        "empty content-type value",
			contentType: "",
			allowedType: contentTypeJSON,
			wantError:   errorInvalidContentType,
		},
		{
			name:        "valid json content-type",
			contentType: contentTypeJSON,
			allowedType: contentTypeJSON,
			wantError:   jsonResponse{},
		},
		{
			name:        "json with charset",
			contentType: contentTypeJSONWithCharset,
			allowedType: contentTypeJSON,
			wantError:   jsonResponse{},
		},
		{
			name:        "json with extra parameters",
			contentType: contentTypeJSONWithParams,
			allowedType: contentTypeJSON,
			wantError:   jsonResponse{},
		},
		{
			name:        "invalid content-type",
			contentType: contentTypePlainText,
			allowedType: contentTypeJSON,
			wantError:   errorInvalidContentType,
		},
	}

	validator := NewValidator()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}

			resp, err := validator.ContentType(req, tc.allowedType)

			if tc.wantError.status == 0 {
				// Expect success case
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if resp.status != 0 || len(resp.body) != 0 {
					t.Errorf("expected empty jsonResponse, got %+v", resp)
				}
			} else {
				// Expect error case
				if err == nil {
					t.Error("expected an error, got nil")
				}
				if resp.status != tc.wantError.status {
					t.Errorf("expected status %d, got %d", tc.wantError.status, resp.status)
				}
				if string(resp.body) != string(tc.wantError.body) {
					t.Errorf("expected error response body %q, got %q", string(tc.wantError.body), string(resp.body))
				}
			}
		})
	}
}

func TestEmailValidation(t *testing.T) {
	testCases := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid email with subdomain", "test@sub.example.com", false},
		{"invalid email no at", "test.example.com", true},
		{"invalid email no domain", "test@", true},
		{"invalid email with spaces", "test @example.com", true},
		{"empty email", "", true},
	}

	validator := NewValidator()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validator.Email(tc.email)
			if (err != nil) != tc.wantErr {
				t.Errorf("Email() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestPasswordValidation(t *testing.T) {
	testCases := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid password", "secure-password-123", false},
		{"too short", "short", true},
		{"exactly 8 chars", "1234567a", false},
		{"exactly 72 bytes", "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrtu", false},
		{"too long (73 bytes)", "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrtuv", true},
		{"too long with multi-byte (73 bytes)", "世界世界世界世界世界世界世界世界世界世界世界世界世界世界世界世界世界世界世界世界世界世界世界!", true},
		{"contains null byte", "pass\x00word", true},
		{"common password", "password", true},
		{"common password case insensitive", "Password", true},
	}

	validator := NewValidator()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validator.Password(tc.password)
			if (err != nil) != tc.wantErr {
				t.Errorf("Password(%q) error = %v, wantErr %v", tc.password, err, tc.wantErr)
			}
		})
	}
}
