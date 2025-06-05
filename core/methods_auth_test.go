package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func TestAuthenticateRequestValidation(t *testing.T) {
	testCases := []struct {
		name       string
		authHeader string
		wantError  *jsonError // This represents the expected jsonResponse
	}{
		{
			name:       "missing authorization header",
			authHeader: "",
			wantError:  &errorNoAuthHeader,
		},
		{
			name:       "invalid token format",
			authHeader: "InvalidToken",
			wantError:  &errorInvalidTokenFormat,
		},
		{
			name:       "invalid bearer prefix",
			authHeader: "Basic abc123",
			wantError:  &errorInvalidTokenFormat,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			// No need for ResponseRecorder or middleware setup, as Authenticate is a direct function call
			// rr := httptest.NewRecorder() // Removed

			a, _ := New(
				WithConfig(&config.Config{
					Jwt: config.Jwt{
						AuthSecret:        []byte("test_secret_32_bytes_long_xxxxxx"),
						AuthTokenDuration: 15 * time.Minute,
					},
				}),
				WithDB(&MockDB{}),
				WithRouter(&MockRouter{}),
			)

			// Directly call the Authenticate method
			user, authErr, resp := a.Authenticate(req)

			// Assert on the jsonResponse returned by Authenticate
			if resp.status != tc.wantError.status {
				t.Errorf("expected status %d, got %d", tc.wantError.status, resp.status)
			}
			if string(resp.body) != string(tc.wantError.body) {
				t.Errorf("expected error response body %q, got %q", string(tc.wantError.body), string(resp.body))
			}
		})
	}
}

