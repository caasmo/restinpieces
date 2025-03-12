package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func TestJwtValidateMiddleware(t *testing.T) {
	testCases := []struct {
		name         string
		authHeader   string
		wantError    *jsonError
		expectUserID bool
	}{
		{
			name:         "valid token",
			authHeader:   "Bearer " + generateTestToken(t, "testuser123"),
			wantError:    nil,
			expectUserID: true,
		},
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
			name:       "expired token",
			authHeader: "Bearer " + generateExpiredTestToken(t, "testuser123"),
			wantError:  &errorJwtTokenExpired,
		},
		{
			name:       "invalid signing method",
			authHeader: "Bearer " + generateInvalidSigningToken(t, "testuser123"),
			wantError:  &errorJwtInvalidSignMethod,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			rr := httptest.NewRecorder()
			a, _ := New(
				WithConfig(&config.Config{
					JwtSecret:     []byte("test_secret_32_bytes_long_xxxxxx"), // 32-byte secret
					TokenDuration: 15 * time.Minute,
				}),
				WithDB(&MockDB{}),
				WithRouter(&MockRouter{}),
			)

			// Create a test handler that checks for user ID in context
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				userID, ok := r.Context().Value(UserIDKey).(string)
				if tc.expectUserID && !ok {
					t.Error("Expected user ID in context but none found")
				}
				_ = userID // Silence unused var check
				w.WriteHeader(http.StatusOK)
			})

			// Apply the middleware and serve the request
			middleware := a.JwtValidate(testHandler)
			middleware.ServeHTTP(rr, req)

			if tc.wantError != nil {
				if rr.Code != tc.wantError.code {
					t.Errorf("expected status %d, got %d", tc.wantError.code, rr.Code)
				}
				if rr.Body.String() != string(tc.wantError.body) {
					t.Errorf("expected error response %q, got %q", string(tc.wantError.body), rr.Body.String())
				}
			} else {
				if rr.Code != http.StatusOK {
					t.Errorf("expected status OK, got %d", rr.Code)
				}
			}
		})
	}
}

func generateTestToken(t *testing.T, userID string) string {
	t.Helper()

    // jwt.MapClaims is just map[string]any
	claims := map[string]any{crypto.ClaimUserID: userID}

	token, _, err := crypto.NewJwt(claims, []byte("test_secret_32_bytes_long_xxxxxx"), 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}

func generateExpiredTestToken(t *testing.T, userID string) string {
	t.Helper()

	claims := map[string]any{crypto.ClaimUserID: userID}
	token, _, err := crypto.NewJwt(claims, []byte("test_secret_32_bytes_long_xxxxxx"), -30*time.Minute) // Negative duration for expired token
	if err != nil {
		t.Fatalf("failed to generate expired test token: %v", err)
	}
	return token
}

func generateInvalidSigningToken(t *testing.T, userID string) string {
	t.Helper()
	// Create token with invalid signing method (ES256 instead of HMAC)
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodES256, jwtv5.MapClaims{
		crypto.ClaimUserID:   userID,
		crypto.ClaimExpiresAt: jwtv5.NewNumericDate(time.Now().Add(15 * time.Minute)),
	})

	// Generate EC key pair for testing
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate EC key: %v", err)
	}

	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenString
}
