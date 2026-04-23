package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/db/mock"
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
			wantError:  errorNoAuthHeader,
		},
		{
			name:       "invalid bearer prefix",
			authHeader: "Basic abc123",
			wantError:  errorNoAuthHeader,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			mockDB := &mock.Db{} // Create a mock DB instance

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
		ID:       "r1a2b3c4d5e6f70",
		Email:    "test@example.com",
		Password: "hashed_password",
		Verified: true,
	}

	testCases := []struct {
		name       string
		userSetup  func(*mock.Db)
		tokenSetup func(*testing.T) string
		wantError  jsonResponse
	}{
		{
			name: "invalid signing method",
			userSetup: func(mockDB *mock.Db) {
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
			userSetup: func(mockDB *mock.Db) {
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
			userSetup: func(mockDB *mock.Db) {
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
			userSetup: func(mockDB *mock.Db) {
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
			userSetup: func(mockDB *mock.Db) {
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
			mockDB := &mock.Db{}
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
				if user != nil && user.ID != testUser.ID {
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
	// Including uid_mac so it passes the MAC gatekeeper before being
	// rejected at the signature verification step.
	now := time.Now()
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodES256, jwtv5.MapClaims{
		crypto.ClaimUserID: userID,
		crypto.ClaimUidMac: crypto.GenerateUserMac(userID, "test_secret_32_bytes_long_xxxxxx"),
		"iat":              now.Unix(),
		"exp":              now.Add(15 * time.Minute).Unix(),
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
	claims := map[string]any{
		crypto.ClaimUserID: "r1a2b3c4d5e6f70", // Use fixed test user ID
		crypto.ClaimUidMac: crypto.GenerateUserMac("r1a2b3c4d5e6f70", secret),
	}
	token, err := crypto.NewJwt(claims, signingKey, expiresIn)
	if err != nil {
		return "", fmt.Errorf("failed to generate test token: %w", err)
	}
	return token, nil
}

func TestNewDefaultAuthenticator(t *testing.T) {
	mockDB := &mock.Db{}
	logger := slog.Default()
	cfg := &config.Config{}
	configProvider := config.NewProvider(cfg)

	auth := NewDefaultAuthenticator(mockDB, logger, configProvider)

	if auth.dbAuth != mockDB {
		t.Error("dbAuth not set correctly")
	}
	if auth.logger != logger {
		t.Error("logger not set correctly")
	}
	if auth.configProvider != configProvider {
		t.Error("configProvider not set correctly")
	}
}

func TestAuthenticateErrorCases(t *testing.T) {
	testUser := &db.User{
		ID:       "r1a2b3c4d5e6f70",
		Email:    "test@example.com",
		Password: "hashed_password",
	}

	testCases := []struct {
		name      string
		userSetup func(*mock.Db)
		token     string
		secret    string
		wantError jsonResponse
	}{
		{
			name:      "unverified parse error",
			userSetup: nil,
			token:     "invalid.token.string",
			secret:    "test_secret_32_bytes_long_xxxxxx",
			wantError: errorJwtInvalidToken,
		},
		{
			name:      "session validation error",
			userSetup: nil,
			token: func() string {
				claims := jwtv5.MapClaims{
					crypto.ClaimUserID: "r1a2b3c4d5e6f70",
					// Missing iat and exp
				}
				token, _ := crypto.NewJwt(claims, []byte("test_secret_32_bytes_long_xxxxxx"), 15*time.Minute)
				return token
			}(),
			secret:    "test_secret_32_bytes_long_xxxxxx",
			wantError: errorJwtInvalidToken,
		},
		{
			name: "signing key creation error",
			userSetup: func(mockDB *mock.Db) {
				mockDB.GetUserByIdFunc = func(id string) (*db.User, error) {
					return testUser, nil
				}
			},
			token: func() string {
				token, _ := generateToken(testUser.Email, testUser.Password, "a_different_secret_that_is_long_enough", 15*time.Minute)
				return token
			}(),
			secret:    "short", // MAC check fails (different secret) → errorJwtInvalidToken
			wantError: errorJwtInvalidToken,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := &mock.Db{}
			if tc.userSetup != nil {
				tc.userSetup(mockDB)
			}

			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)

			cfg := &config.Config{
				Jwt: config.Jwt{
					AuthSecret:        tc.secret,
					AuthTokenDuration: config.Duration{Duration: 15 * time.Minute},
				},
			}
			configProvider := config.NewProvider(cfg)

			auth := NewDefaultAuthenticator(mockDB, slog.Default(), configProvider)
			user, resp, authErr := auth.Authenticate(req)

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
		})
	}
}

func TestAuthenticateSessionClaimsValidation(t *testing.T) {
	testUser := &db.User{
		ID:       "r1a2b3c4d5e6f70",
		Email:    "test@example.com",
		Password: "hashed_password",
	}
	secret := "test_secret_32_bytes_long_xxxxxx"

	// Helper to generate a token with custom claims
	generateTestToken := func(claims jwtv5.MapClaims) string {
		signingKey, err := crypto.NewJwtSigningKeyWithCredentials(testUser.Email, testUser.Password, secret)
		if err != nil {
			t.Fatalf("failed to create signing key: %v", err)
		}
		token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(signingKey)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return tokenString
	}

	testCases := []struct {
		name   string
		claims jwtv5.MapClaims
	}{
		{
			name: "missing user_id claim",
			claims: jwtv5.MapClaims{
				"iat": time.Now().Unix(),
				"exp": time.Now().Add(15 * time.Minute).Unix(),
			},
		},
		{
			name: "empty user_id claim",
			claims: jwtv5.MapClaims{
				crypto.ClaimUserID: "",
				"iat":              time.Now().Unix(),
				"exp":              time.Now().Add(15 * time.Minute).Unix(),
			},
		},
		{
			name: "missing iat claim",
			claims: jwtv5.MapClaims{
				crypto.ClaimUserID: "r1a2b3c4d5e6f70",
				"exp":              time.Now().Add(15 * time.Minute).Unix(),
			},
		},
		{
			name: "missing exp claim",
			claims: jwtv5.MapClaims{
				crypto.ClaimUserID: "r1a2b3c4d5e6f70",
				"iat":              time.Now().Unix(),
			},
		},
		{
			name: "iat in the future",
			claims: jwtv5.MapClaims{
				crypto.ClaimUserID: "r1a2b3c4d5e6f70",
				"iat":              time.Now().Add(10 * time.Minute).Unix(),
				"exp":              time.Now().Add(25 * time.Minute).Unix(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := &mock.Db{
				GetUserByIdFunc: func(id string) (*db.User, error) {
					return testUser, nil
				},
			}
			cfg := &config.Config{
				Jwt: config.Jwt{
					AuthSecret:        secret,
					AuthTokenDuration: config.Duration{Duration: 15 * time.Minute},
				},
			}
			configProvider := config.NewProvider(cfg)
			auth := NewDefaultAuthenticator(mockDB, slog.Default(), configProvider)

			req := httptest.NewRequest("GET", "/protected", nil)
			token := generateTestToken(tc.claims)
			req.Header.Set("Authorization", "Bearer "+token)

			user, resp, authErr := auth.Authenticate(req)

			if user != nil {
				t.Errorf("expected user to be nil, got %v", user)
			}
			if authErr == nil {
				t.Fatal("expected an authentication error, got nil")
			}
			// All session validation errors should map to errorJwtInvalidToken
			if resp.status != errorJwtInvalidToken.status {
				t.Errorf("expected status %d, got %d", errorJwtInvalidToken.status, resp.status)
			}
			if string(resp.body) != string(errorJwtInvalidToken.body) {
				t.Errorf("expected error response body %q, got %q", string(errorJwtInvalidToken.body), string(resp.body))
			}
		})
	}
}

func TestExtractAndVerifyUserID(t *testing.T) {
	secret := "test_secret_32_bytes_long_xxxxxx"
	validUserID := "r1a2b3c4d5e6f70"

	// buildToken constructs a JWT-like string with the given claims in the payload.
	// The header and signature are placeholders — extractAndVerifyUserID only reads
	// the payload (middle segment) and never verifies the signature.
	buildToken := func(claims map[string]string) string {
		payload := make(map[string]any)
		for k, v := range claims {
			payload[k] = v
		}
		jsonBytes, _ := json.Marshal(payload)
		encoded := base64.RawURLEncoding.EncodeToString(jsonBytes)
		return "header." + encoded + ".signature"
	}

	validMac := crypto.GenerateUserMac(validUserID, secret)

	testCases := []struct {
		name        string
		tokenString string
		expectedID  string
		expectError bool
	}{
		{
			name: "valid token with valid mac",
			tokenString: buildToken(map[string]string{
				"user_id": validUserID,
				"uid_mac": validMac,
			}),
			expectedID:  validUserID,
			expectError: false,
		},
		{
			name:        "invalid token format",
			tokenString: "invalid.token",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "invalid base64 payload",
			tokenString: "header.invalid-payload.signature",
			expectedID:  "",
			expectError: true,
		},
		{
			name: "missing user_id and uid_mac",
			tokenString: buildToken(map[string]string{
				"some_other_claim": "value",
			}),
			expectedID:  "",
			expectError: true,
		},
		{
			name: "missing uid_mac",
			tokenString: buildToken(map[string]string{
				"user_id": validUserID,
			}),
			expectedID:  "",
			expectError: true,
		},
		{
			name: "missing user_id",
			tokenString: buildToken(map[string]string{
				"uid_mac": validMac,
			}),
			expectedID:  "",
			expectError: true,
		},
		{
			name: "invalid mac format",
			tokenString: buildToken(map[string]string{
				"user_id": validUserID,
				"uid_mac": "not-a-valid-mac",
			}),
			expectedID:  "",
			expectError: true,
		},
		{
			name: "wrong mac for user_id",
			tokenString: buildToken(map[string]string{
				"user_id": "different_user_id",
				"uid_mac": validMac,
			}),
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			userID, err := extractAndVerifyUserID(tc.tokenString, secret)

			if tc.expectError {
				if err == nil {
					t.Error("expected an error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("did not expect an error but got: %v", err)
				}
				if userID != tc.expectedID {
					t.Errorf("expected user ID %q, got %q", tc.expectedID, userID)
				}
			}
		})
	}
}
