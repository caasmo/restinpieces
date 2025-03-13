package app

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

func TestJwtValidate_RequestValidation(t *testing.T) {
	testCases := []struct {
		name       string
		authHeader string
		wantError  *jsonError
	}{
		{
			name:       "invalid signing method",
			authHeader: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE6MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", // Malformed token with invalid time format
			wantError:  &errorJwtInvalidSignMethod,
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

			rr := httptest.NewRecorder()
			a, _ := New(
				WithConfig(&config.Config{
					JwtSecret:     []byte("test_secret_32_bytes_long_xxxxxx"),
					TokenDuration: 15 * time.Minute,
				}),
				WithDB(&MockDB{}),
				WithRouter(&MockRouter{}),
			)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			middleware := a.JwtValidate(testHandler)
			middleware.ServeHTTP(rr, req)

			if rr.Code != tc.wantError.code {
				t.Errorf("expected status %d, got %d", tc.wantError.code, rr.Code)
			}
			if !strings.Contains(rr.Body.String(), string(tc.wantError.body)) {
				t.Errorf("expected error response %q, got %q", string(tc.wantError.body), rr.Body.String())
			}
		})
	}
}

func TestJwtValidate_DatabaseTests(t *testing.T) {
	testUser := &db.User{
		ID:       "testuser123",
		Email:    "test@example.com",
		Password: "hashed_password",
	}

	testCases := []struct {
		name       string
		userSetup  func(*MockDB)
		tokenSetup func(*testing.T) string
		wantError  *jsonError
	}{
		{
			name: "valid token",
			userSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdConfig.User = testUser
				// Set up user's email and password to match expected signing key
				//testUser.Email = "test@example.com"
				//testUser.Password = "hashed_password"
			},
			tokenSetup: func(t *testing.T) string {
				token, err := generateToken(testUser.Email, testUser.Password, []byte("test_secret_32_bytes_long_xxxxxx"), 15*time.Minute)
				if err != nil {
					t.Fatalf("failed to generate token: %v", err)
				}
				return token
			},
			wantError: nil,
		},
		{
			name: "expired token",
			userSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdConfig.User = testUser
			},
			tokenSetup: func(t *testing.T) string {
				token, err := generateToken(testUser.Email, testUser.Password, []byte("test_secret_32_bytes_long_xxxxxx"), -30*time.Minute)
				if err != nil {
					t.Fatalf("failed to generate token: %v", err)
				}
				return token
			},
			wantError: &errorJwtTokenExpired,
		},
		{
			name: "user not found",
			userSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdConfig.User = nil
			},
			tokenSetup: func(t *testing.T) string {
				token, err := generateToken(testUser.Email, testUser.Password, []byte("test_secret_32_bytes_long_xxxxxx"), 15*time.Minute)
				if err != nil {
					t.Fatalf("failed to generate token: %v", err)
				}
				return token
			},
			wantError: &errorJwtInvalidToken,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := &MockDB{}
			if tc.userSetup != nil {
				tc.userSetup(mockDB)
			}

			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Bearer " + tc.tokenSetup(t))

			rr := httptest.NewRecorder()
			a, _ := New(
				WithConfig(&config.Config{
					JwtSecret:     []byte("test_secret_32_bytes_long_xxxxxx"),
					TokenDuration: 15 * time.Minute,
				}),
				WithDB(mockDB),
				WithRouter(&MockRouter{}),
			)

			var capturedUserID string
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedUserID = r.Context().Value(UserIDKey).(string)
				w.WriteHeader(http.StatusOK)
			})

			middleware := a.JwtValidate(testHandler)
			middleware.ServeHTTP(rr, req)

			if tc.wantError != nil {
				if rr.Code != tc.wantError.code {
					t.Errorf("expected status %d, got %d", tc.wantError.code, rr.Code)
				}
				if !strings.Contains(rr.Body.String(), string(tc.wantError.body)) {
					t.Errorf("expected error response %q, got %q", string(tc.wantError.body), rr.Body.String())
				}
			} else {
				if rr.Code != http.StatusOK {
					t.Errorf("expected status OK, got %d", rr.Code)
				}
				if capturedUserID != testUser.ID {
					t.Errorf("expected user ID %q in context, got %q", testUser.ID, capturedUserID)
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

func generateToken(email, passwordHash string, secret []byte, expiresIn time.Duration) (string, error) {
	// Generate signing key using user credentials and secret
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to generate signing key: %w", err)
	}

	// Generate token with derived signing key
	claims := map[string]any{crypto.ClaimUserID: "testuser123"} // Use fixed test user ID
	token, _, err := crypto.NewJwt(claims, signingKey, expiresIn)
	if err != nil {
		return "", fmt.Errorf("failed to generate test token: %w", err)
	}
	return token, nil
}

