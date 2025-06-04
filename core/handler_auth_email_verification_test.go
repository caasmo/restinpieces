package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
)

func TestRequestVerificationHandlerRequestValidation(t *testing.T) {
	testCases := []struct {
		name       string
		json       string
		wantStatus int
	}{
		{
			name:       "invalid email format",
			json:       `{"email":"not-an-email"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty email",
			json:       `{"email":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing email field",
			json:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			json:       `{"email": invalid}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty email",
			json:       `{"email":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing email field",
			json:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			json:       `{"email": invalid}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := tc.json
			req := httptest.NewRequest("POST", "/request-verification", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			a, _ := New(
				WithConfig(&config.Config{
					Jwt: config.Jwt{
						AuthSecret:        []byte("test_secret_32_bytes_long_xxxxxx"),
						AuthTokenDuration: 15 * time.Minute,
					},
				}),
				WithRouter(&MockRouter{}),
			)

			a.RequestVerificationHandler(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}

			// Validate error response body when expected
			if tc.wantStatus >= 400 {
				var resp map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode error response: %v", err)
				}

				if _, ok := resp["message"]; !ok {
					t.Error("error response missing 'message' field")
				}
			}
		})
	}
}
