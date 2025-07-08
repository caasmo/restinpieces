package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http/httptest"
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
		wantError  jsonResponse // This represents the expected jsonResponse
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

			mockDB := &MockDB{} // Create a mock DB instance

			// Create a config provider
			cfg := &config.Config{
				Jwt: config.Jwt{
					AuthSecret:        "test_secret_32_bytes_long_xxxxxx",
					AuthTokenDuration: config.Duration{Duration: 15 * time.Minute},
				},
			}
			configProvider := config.NewProvider(cfg) // Assuming config.NewProvider exists

			// Create authenticator directly
			auth := NewDefaultAuthenticator(mockDB, slog.Default(), configProvider)
			user, resp, authErr := auth.Authenticate(req)

			// Assert that user is nil for these error cases
			if user != nil {
				t.Errorf("expected user to be nil, got %v", user)
			}

			// Assert that authErr is not nil (it's always "Auth error" for security)
			if authErr == nil {
				t.Error("expected an authentication error, got nil")
			}

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

func TestAuthenticateToken(t *testing.T) {
	testUser := &db.User{
		ID:       "testuser123",
		Email:    "test@example.com",
		Password: "hashed_password",
	}

	testCases := []struct {
		name       string
		userSetup  func(*MockDB)
		tokenSetup func(*testing.T) string
		wantError  jsonResponse
	}{
		{
			name: "invalid signing method",
			userSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdFunc = func(id string) (*db.User, error) {
					return testUser, nil
				}
			},
			tokenSetup: func(t *testing.T) string {
				token, err := generateES256Token(testUser.ID)
				if err != nil {
					t.Fatalf("failed to generate ES256 token: %v", err)
				}
				return token
			},
			wantError: errorJwtInvalidSignMethod,
		},
		{
			name: "valid token",
			userSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdFunc = func(id string) (*db.User, error) { // Use func field
					return testUser, nil
				}
			},
			tokenSetup: func(t *testing.T) string {
				token, err := generateToken(testUser.Email, testUser.Password, "test_secret_32_bytes_long_xxxxxx", 15*time.Minute)
				if err != nil {
					t.Fatalf("failed to generate token: %v", err)
				}
				return token
			},
			wantError: jsonResponse{},
		},
		{
			name: "expired token",
			userSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdFunc = func(id string) (*db.User, error) { // Use func field
					return testUser, nil
				}
			},
			tokenSetup: func(t *testing.T) string {
				token, err := generateToken(testUser.Email, testUser.Password, "test_secret_32_bytes_long_xxxxxx", -30*time.Minute)
				if err != nil {
					t.Fatalf("failed to generate token: %v", err)
				}
				return token
			},
			wantError: errorJwtTokenExpired,
		},
		{
			name: "user not found",
			userSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdFunc = func(id string) (*db.User, error) { // Use func field
					return nil, db.ErrUserNotFound // Simulate user not found
				}
			},
			tokenSetup: func(t *testing.T) string {
				token, err := generateToken(testUser.Email, testUser.Password, "test_secret_32_bytes_long_xxxxxx", 15*time.Minute)
				if err != nil {
					t.Fatalf("failed to generate token: %v", err)
				}
				return token
			},
			wantError: errorJwtInvalidToken,
		},
		{
			name: "database error on GetUserById",
			userSetup: func(mockDB *MockDB) {
				mockDB.GetUserByIdFunc = func(id string) (*db.User, error) { // Use func field
					return nil, errors.New("database error") // Simulate database error
				}
			},
			tokenSetup: func(t *testing.T) string {
				token, err := generateToken(testUser.Email, testUser.Password, "test_secret_32_bytes_long_xxxxxx", 15*time.Minute)
				if err != nil {
					t.Fatalf("failed to generate token: %v", err)
				}
				return token
			},
			wantError: errorJwtInvalidToken, // Authenticate maps DB errors to generic invalid token
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

			// Create a config provider
			cfg := &config.Config{
				Jwt: config.Jwt{
					AuthSecret:        "test_secret_32_bytes_long_xxxxxx",
					AuthTokenDuration: config.Duration{Duration: 15 * time.Minute},
				},
			}
			configProvider := config.NewProvider(cfg)

			// Create authenticator directly
			auth := NewDefaultAuthenticator(mockDB, slog.Default(), configProvider)
			user, resp, authErr := auth.Authenticate(req)

			if tc.wantError.status != 0 {
				// Expect an error case
				if user != nil {
					t.Errorf("expected user to be nil, got %v", user)
				}
				if authErr == nil {
					t.Error("expected an authentication error, got nil")
				}
				if resp.status != tc.wantError.status {
					t.Errorf("expected status %d, got %d", tc.wantError.status, resp.status)
				}
				if string(resp.body) != string(tc.wantError.body) {
					t.Errorf("expected error response body %q, got %q", string(tc.wantError.body), string(resp.body))
				}
			} else {
				// Expect success case
				if user == nil {
					t.Error("expected a user, got nil")
				}
				if authErr != nil {
					t.Errorf("expected no authentication error, got %v", authErr)
				}
				if resp.status != 0 || len(resp.body) != 0 { // jsonResponse{} is zero value for success
					t.Errorf("expected empty jsonResponse, got status %d, body %q", resp.status, string(resp.body))
				}
				if user.ID != testUser.ID {
					t.Errorf("expected authenticated user ID %q, got %q", testUser.ID, user.ID)
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

func generateToken(email, passwordHash string, secret string, expiresIn time.Duration) (string, error) {
	// Generate signing key using user credentials and secret
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to generate signing key: %w", err)
	}

	// Generate token with derived signing key
	claims := map[string]any{crypto.ClaimUserID: "testuser123"} // Use fixed test user ID
	token, err := crypto.NewJwt(claims, signingKey, expiresIn)
	if err != nil {
		return "", fmt.Errorf("failed to generate test token: %w", err)
	}
	return token, nil
}
