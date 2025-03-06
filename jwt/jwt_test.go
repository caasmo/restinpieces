package jwt

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCreateAndParseValidToken(t *testing.T) {
	secret := []byte("test_secret_32_bytes_long_xxxxxx")
	userID := "testuser123"
	tokenDuration := 15 * time.Minute

	tokenString, _, err := Create(userID, secret, tokenDuration)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	claims, err := Parse(tokenString, secret)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected UserID %q, got %q", userID, claims.UserID)
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
			secret:      []byte("test_secret"),
			wantError:   ErrTokenExpired,
		},
		{
			name:        "invalid signature",
			tokenString: generateValidToken(t),
			secret:      []byte("wrong_secret"),
			wantError:   ErrInvalidToken,
		},
		{
			name:        "invalid signing method",
			tokenString: generateES256Token(t),
			secret:      []byte("test_secret"),
			wantError:   ErrInvalidSigningMethod,
		},
		{
			name:        "malformed token",
			tokenString: "malformed.token.string",
			secret:      []byte("test_secret"),
			wantError:   ErrInvalidToken,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.tokenString, tc.secret)
			if !errors.Is(err, tc.wantError) {
				t.Errorf("Parse() error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

func TestCreateWithInvalidSecret(t *testing.T) {
	_, _, err := Create("user123", nil, 15*time.Minute)
	if err == nil {
		t.Error("expected error when creating token with empty secret")
	}
}

func TestRefreshToken(t *testing.T) {
	secret := []byte("test_secret_32_bytes_long_xxxxxx")
	userID := "testuser456"
	tokenDuration := 15 * time.Minute

	originalToken, originalExpiry, err := Create(userID, secret, tokenDuration)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	refreshedToken, refreshedExpiry, err := Refresh(userID, secret, tokenDuration)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if refreshedExpiry.Before(originalExpiry) {
		t.Error("refreshed token expiry should be later than original")
	}

	// Verify refreshed token is valid
	_, err = Parse(refreshedToken, secret)
	if err != nil {
		t.Errorf("Parse() refreshed token error = %v", err)
	}

	// Verify original token is still valid until expiration
	_, err = Parse(originalToken, secret)
	if err != nil {
		t.Errorf("Parse() original token error = %v", err)
	}
}

func generateValidToken(t *testing.T) string {
	t.Helper()
	token, _, err := Create("testuser", []byte("test_secret_32_bytes_long_xxxxxx"), 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate valid token: %v", err)
	}
	return token
}

func generateExpiredToken(t *testing.T) string {
	t.Helper()
	token, _, err := Create("testuser", []byte("test_secret_32_bytes_long_xxxxxx"), -15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}
	return token
}

func generateES256Token(t *testing.T) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, &Claims{
		UserID: "testuser",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
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
