package crypto

import (
	"crypto/hmac"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCreateAndParseValidToken(t *testing.T) {
	secret := []byte("test_secret_32_bytes_long_xxxxxx")
	userID := "testuser123"
	tokenDuration := 15 * time.Minute

	claims := jwt.MapClaims{"user_id": userID}
	tokenString, err := NewJwt(claims, secret, tokenDuration)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	parsedClaims, err := ParseJwt(tokenString, secret)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if parsedClaims["user_id"] != userID {
		t.Errorf("expected UserID %q, got %q", userID, parsedClaims["user_id"])
	}
}

func TestParseInvalidToken(t *testing.T) {
	testCases := []struct {
		name        string
		tokenString string
		secret      []byte
		wantError   error
	}{
		{
			name:        "expired token",
			tokenString: generateExpiredToken(t),
			secret:      []byte("test_secret_32_bytes_long_xxxxxx"),
			wantError:   ErrJwtTokenExpired,
		},
		{
			name:        "invalid signature",
			tokenString: generateValidToken(t),
			secret:      []byte("wrong_secret"),
			wantError:   ErrJwtInvalidSigningMethod,
		},
		{
			name:        "invalid signing method",
			tokenString: generateES256Token(t),
			secret:      []byte("test_secret"),
			wantError:   ErrJwtInvalidSigningMethod,
		},
		{
			name:        "malformed token",
			tokenString: "malformed.token.string",
			secret:      []byte("test_secret"),
			wantError:   ErrJwtInvalidToken,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseJwt(tc.tokenString, tc.secret)
			if !errors.Is(err, tc.wantError) {
				t.Errorf("Parse() error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

func TestCreateWithInvalidSecret(t *testing.T) {
	claims := jwt.MapClaims{"user_id": "user123"}
	_, err := NewJwt(claims, nil, 15*time.Minute)
	if !errors.Is(err, ErrJwtInvalidSecretLength) {
		t.Errorf("expected ErrInvalidSecretLength, got %v", err)
	}
}

func generateValidToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{"user_id": "testuser"}
	token, err := NewJwt(claims, []byte("test_secret_32_bytes_long_xxxxxx"), 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate valid token: %v", err)
	}
	return token
}

func generateExpiredToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{"user_id": "testuser"}
	token, err := NewJwt(claims, []byte("test_secret_32_bytes_long_xxxxxx"), -15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}
	return token
}

func TestParseJwtUnverified(t *testing.T) {
	testCases := []struct {
		name        string
		tokenString string
		wantClaims  jwt.MapClaims
		wantError   error
	}{
		{
			name:        "valid token",
			tokenString: generateValidToken(t),
			wantClaims:  jwt.MapClaims{"user_id": "testuser"},
			wantError:   nil,
		},
		{
			name:        "malformed token",
			tokenString: "malformed.token.string",
			wantClaims:  nil,
			wantError:   jwt.ErrTokenMalformed,
		},
		{
			name:        "invalid token format",
			tokenString: "invalid.token",
			wantClaims:  nil,
			wantError:   jwt.ErrTokenMalformed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			claims, err := ParseJwtUnverified(tc.tokenString)

			// Check error expectations
			if (err != nil && tc.wantError == nil) || (err == nil && tc.wantError != nil) {
				t.Errorf("ParseJwtUnverified() error = %v, want %v", err, tc.wantError)
				return
			}

			// Check claims expectations
			if tc.wantClaims != nil {
				if claims == nil {
					t.Error("expected non-nil claims, got nil")
					return
				}

				for k, v := range tc.wantClaims {
					if claims[k] != v {
						t.Errorf("expected claim %q = %v, got %v", k, v, claims[k])
					}
				}
			} else if claims != nil {
				t.Error("expected nil claims, got non-nil")
			}
		})
	}
}

func TestNewJwtSigningKeyWithCredentials(t *testing.T) {
	validSecret := []byte("test_secret_32_bytes_long_xxxxxx")
	testEmail := "test@example.com"
	testCases := []struct {
		name      string
		email     string
		password  string
		wantError error
	}{
		{
			name:      "with password",
			email:     testEmail,
			password:  "hashed_password_123",
			wantError: nil,
		},
		{
			name:      "without password",
			email:     testEmail,
			password:  "",
			wantError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key1, err := NewJwtSigningKeyWithCredentials(tc.email, tc.password, string(validSecret))
			if !errors.Is(err, tc.wantError) {
				t.Fatalf("NewJwtSigningKeyWithCredentials() error = %v, want %v", err, tc.wantError)
			}

			if len(key1) != 32 { // SHA256 hash length
				t.Errorf("key length = %d, want 32", len(key1))
			}

			// Verify deterministic output
			key2, err := NewJwtSigningKeyWithCredentials(tc.email, tc.password, string(validSecret))
			if err != nil {
				t.Fatalf("Second call failed unexpectedly: %v", err)
			}
			if !hmac.Equal(key1, key2) {
				t.Error("returned different keys for same inputs")
			}
		})
	}
}

func TestNewJwtSigningKeyWithCredentialsErrors(t *testing.T) {
	validSecret := []byte("test_secret_32_bytes_long_xxxxxx")
	testEmail := "test@example.com"

	tests := []struct {
		name      string
		email     string
		password  string
		secret    []byte
		wantError error
	}{
		{
			name:      "empty email",
			email:     "",
			password:  "hashed_password_123",
			secret:    validSecret,
			wantError: ErrInvalidSigningKeyParts,
		},
		{
			name:      "empty password hash",
			email:     testEmail,
			password:  "",
			secret:    validSecret,
			wantError: nil,
		},
		{
			name:      "short server secret",
			email:     testEmail,
			password:  "hashed_password_123",
			secret:    []byte("short"),
			wantError: ErrJwtInvalidSecretLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewJwtSigningKeyWithCredentials(tt.email, tt.password, string(tt.secret))
			if !errors.Is(err, tt.wantError) {
				t.Errorf("NewJwtSigningKeyWithCredentials() error = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func generateES256Token(t *testing.T) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"user_id": "testuser",
		"exp":     jwt.NewNumericDate(time.Now().Add(15 * time.Minute)).Unix(),
	})
	privateKey, err := jwt.ParseECPrivateKeyFromPEM([]byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIOVgr0F8R8V7+ZGuDGpckbGexDqzu8lQw0Aehp7RVfWRoAoGCCqGSM49
AwEHoUQDQgAEVHBQ4Q77cjFdbe6y2WbgR4J5l3jZVY6lj4lF4vJQHKRX1Xl3J6HZ
Vdo6H3z/uB1sD6l0HqBz1Y8e+9q9q3X7PA==
-----END EC PRIVATE KEY-----`))
	if err != nil {
		t.Fatalf("failed to parse EC private key: %v", err)
	}
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenString
}
