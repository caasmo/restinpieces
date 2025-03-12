package crypto

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

	claims := jwt.MapClaims{"user_id": userID}
	tokenString, _, err := NewJWT(claims, string(secret), tokenDuration)
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
	_, _, err := NewJWT(claims, "", 15*time.Minute)
	if !errors.Is(err, ErrJwtInvalidSecretLength) {
		t.Errorf("expected ErrInvalidSecretLength, got %v", err)
	}
}

func generateValidToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{"user_id": "testuser"}
	token, _, err := NewJWT(claims, "test_secret_32_bytes_long_xxxxxx", 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate valid token: %v", err)
	}
	return token
}

func generateExpiredToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{"user_id": "testuser"}
	token, _, err := NewJWT(claims, "test_secret_32_bytes_long_xxxxxx", -15*time.Minute)
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
