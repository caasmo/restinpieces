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
		name         string
		contentType  string
		allowedType  string
		wantError    jsonResponse
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

			err, resp := validator.ContentType(req, tc.allowedType)

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
