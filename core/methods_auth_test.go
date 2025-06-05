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
			wantError:  errorNoAuthHeader,
		},
		{
			name:       "invalid token format",
			authHeader: "InvalidToken",
			wantError:  errorInvalidTokenFormat,
		},
		{
			name:       "invalid bearer prefix",
			authHeader: "Basic abc123",
			wantError:  errorInvalidTokenFormat,
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

func TestAuthenticateDatabase(t *testing.T) {
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
			name: "invalid signing method",
			userSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdConfig.User = testUser
			},
			tokenSetup: func(t *testing.T) string {
				token, err := generateES256Token(testUser.ID)
				if err != nil {
					t.Fatalf("failed to generate ES256 token: %v", err)
				}
				return token
			},
			wantError: &errorJwtInvalidSignMethod,
		},
		{
			name: "valid token",
			userSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdConfig.User = testUser
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
			req.Header.Set("Authorization", "Bearer "+tc.tokenSetup(t))

			rr := httptest.NewRecorder()
			a, _ := New(
				WithConfig(&config.Config{
					Jwt: config.Jwt{
						AuthSecret:        []byte("test_secret_32_bytes_long_xxxxxx"),
						AuthTokenDuration: 15 * time.Minute,
					},
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
				if rr.Code != tc.wantError.status {
					t.Errorf("expected status %d, got %d", tc.wantError.status, rr.Code)
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

func generateES256Token(userID string) (string, error) {
	// Generate EC private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate EC key: %w", err)
	}

	// Create token with ES256 signing method and proper claims
	now := time.Now()
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodES256, jwtv5.MapClaims{
		"user_id": userID,
		"iat":     now.Unix(),
		"exp":     now.Add(15 * time.Minute).Unix(),
	})

	// Sign the token
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
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
