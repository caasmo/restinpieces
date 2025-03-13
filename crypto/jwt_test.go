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
	tokenString, _, err := NewJwt(claims, secret, tokenDuration)
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
	_, _, err := NewJwt(claims, nil, 15*time.Minute)
	if !errors.Is(err, ErrJwtInvalidSecretLength) {
		t.Errorf("expected ErrInvalidSecretLength, got %v", err)
	}
}

func generateValidToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{"user_id": "testuser"}
	token, _, err := NewJwt(claims, []byte("test_secret_32_bytes_long_xxxxxx"), 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate valid token: %v", err)
	}
	return token
}

func generateExpiredToken(t *testing.T) string {
	t.Helper()
	claims := jwt.MapClaims{"user_id": "testuser"}
	token, _, err := NewJwt(claims, []byte("test_secret_32_bytes_long_xxxxxx"), -15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}
	return token
}

func TestValidateClaimUserID(t *testing.T) {
	testCases := []struct {
		name      string
		claims    jwt.MapClaims
		wantError error
	}{
		{
			name:      "valid user_id",
			claims:    jwt.MapClaims{ClaimUserID: "user123"},
			wantError: nil,
		},
		{
			name:      "missing user_id in empty claims",
			claims:    jwt.MapClaims{},
			wantError: ErrClaimNotFound,
		},
		{
			name:      "missing user_id in non-empty claims", 
			claims:    jwt.MapClaims{"foo": "bar"},
			wantError: ErrClaimNotFound,
		},
		{
			name:      "user_id as number",
			claims:    jwt.MapClaims{ClaimUserID: 123},
			wantError: ErrInvalidClaimFormat,
		},
		{
			name:      "empty user_id string",
			claims:    jwt.MapClaims{ClaimUserID: ""},
			wantError: ErrInvalidClaimFormat,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateClaimUserID(tc.claims)
			if !errors.Is(err, tc.wantError) {
				t.Errorf("ValidateClaimUserID() error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

func TestValidateClaimIssuedAt(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		claims    jwt.MapClaims
		wantError error
	}{
		{
			name:      "valid iat",
			claims:    jwt.MapClaims{ClaimIssuedAt: now.Add(-1 * time.Minute).Unix()},
			wantError: nil,
		},
		{
			name:      "missing iat",
			claims:    jwt.MapClaims{},
			wantError: ErrClaimNotFound,
		},
		{
			name:      "iat in future",
			claims:    jwt.MapClaims{ClaimIssuedAt: now.Add(1 * time.Minute).Unix()},
			wantError: ErrTokenUsedBeforeIssued,
		},
		{
			name:      "invalid iat type",
			claims:    jwt.MapClaims{ClaimIssuedAt: "not a number"},
			wantError: ErrInvalidClaimFormat,
		},
		{
			name:      "iat in non-empty claims",
			claims:    jwt.MapClaims{"foo": "bar", ClaimIssuedAt: now.Add(-1 * time.Minute).Unix()},
			wantError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateClaimIssuedAt(tc.claims)
			if !errors.Is(err, tc.wantError) {
				t.Errorf("ValidateClaimIssuedAt() error = %v, want %v", err, tc.wantError)
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
