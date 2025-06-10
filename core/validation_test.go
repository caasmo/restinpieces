package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContentTypeValidation(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		wantError   jsonResponse
	}{
		{
			name:        "no content-type header",
			contentType: "",
			wantError:   errorInvalidContentType,
		},
		{
			name:        "empty content-type value",
			contentType: "",
			wantError:   errorInvalidContentType,
		},
		{
			name:        "valid json content-type",
			contentType: "application/json",
			wantError:   jsonResponse{},
		},
		{
			name:        "json with charset",
			contentType: "application/json; charset=utf-8",
			wantError:   jsonResponse{},
		},
		{
			name:        "json with extra parameters",
			contentType: "application/json; charset=utf-8; version=1",
			wantError:   jsonResponse{},
		},
		{
			name:        "invalid content-type",
			contentType: "text/plain",
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
