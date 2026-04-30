package crypto

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestOauth2State(t *testing.T) {
	state := Oauth2State()
	if len(state) != Oauth2StateLength {
		t.Errorf("Oauth2State() length = %d, want %d", len(state), Oauth2StateLength)
	}
	for _, char := range state {
		if !strings.ContainsRune(AlphanumericAlphabet, char) {
			t.Errorf("Oauth2State() contains invalid character: %c", char)
		}
	}
}

func TestOauth2CodeVerifier(t *testing.T) {
	verifier := Oauth2CodeVerifier()
	if len(verifier) != OauthCodeVerifierLength {
		t.Errorf("Oauth2CodeVerifier() length = %d, want %d", len(verifier), OauthCodeVerifierLength)
	}
	for _, char := range verifier {
		if !strings.ContainsRune(pkceAlphabet, char) {
			t.Errorf("Oauth2CodeVerifier() contains invalid character: %c", char)
		}
	}
}

func TestS256Challenge(t *testing.T) {
	// Example from RFC 7636
	code := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	expectedChallenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	challenge := S256Challenge(code)
	if challenge != expectedChallenge {
		t.Errorf("S256Challenge() = %s, want %s", challenge, expectedChallenge)
	}
}

func TestValidateCodeVerifier(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid 43 characters",
			input:   strings.Repeat("a", 43),
			wantErr: false,
		},
		{
			name:    "invalid too short",
			input:   strings.Repeat("a", 42),
			wantErr: true,
		},
		{
			name:    "invalid too long",
			input:   strings.Repeat("a", 44),
			wantErr: true,
		},
		{
			name:    "invalid character",
			input:   strings.Repeat("a", 42) + "!",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCodeVerifier(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateCodeVerifier(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestOauth2StateJWT(t *testing.T) {
	secret := "test-secret-32-characters-long-!!!!"
	cv := Oauth2CodeVerifier()
	duration := 1 * time.Minute

	t.Run("successful generation and verification", func(t *testing.T) {
		token, err := NewJwtOauth2StateToken(cv, secret, duration)
		if err != nil {
			t.Fatalf("NewJwtOauth2StateToken failed: %v", err)
		}

		err = VerifyOauth2StateToken(token, cv, secret)
		if err != nil {
			t.Errorf("VerifyOauth2StateToken failed: %v", err)
		}
	})

	t.Run("fails with mismatched code_verifier", func(t *testing.T) {
		token, _ := NewJwtOauth2StateToken(cv, secret, duration)
		wrongCv := Oauth2CodeVerifier()
		err := VerifyOauth2StateToken(token, wrongCv, secret)
		if err == nil {
			t.Error("VerifyOauth2StateToken should have failed with mismatched CV")
		}
	})

	t.Run("fails with expired token", func(t *testing.T) {
		token, _ := NewJwtOauth2StateToken(cv, secret, -1*time.Minute)
		err := VerifyOauth2StateToken(token, cv, secret)
		if err == nil {
			t.Error("VerifyOauth2StateToken should have failed with expired token")
		}
	})

	t.Run("fails with invalid secret length", func(t *testing.T) {
		_, err := NewJwtOauth2StateToken(cv, "short", duration)
		if err != ErrJwtInvalidSecretLength {
			t.Errorf("expected ErrJwtInvalidSecretLength, got %v", err)
		}
	})

	t.Run("fails with invalid token type", func(t *testing.T) {
		// Create a token with a different type claim
		claims := jwt.MapClaims{
			ClaimOauth2CodeVerifierHash: "somehash",
			ClaimType:                   "wrong_type",
		}
		token, _ := NewJwt(claims, []byte(secret), duration)

		err := VerifyOauth2StateToken(token, cv, secret)
		if err != ErrInvalidVerificationToken {
			t.Errorf("expected ErrInvalidVerificationToken, got %v", err)
		}
	})

	t.Run("fails with non-string type claim", func(t *testing.T) {
		claims := jwt.MapClaims{
			ClaimOauth2CodeVerifierHash: "somehash",
			ClaimType:                   123, // not a string
		}
		token, _ := NewJwt(claims, []byte(secret), duration)

		err := VerifyOauth2StateToken(token, cv, secret)
		if err != ErrInvalidVerificationToken {
			t.Errorf("expected ErrInvalidVerificationToken, got %v", err)
		}
	})

	t.Run("fails with missing type claim", func(t *testing.T) {
		claims := jwt.MapClaims{
			ClaimOauth2CodeVerifierHash: "somehash",
		}
		token, _ := NewJwt(claims, []byte(secret), duration)

		err := VerifyOauth2StateToken(token, cv, secret)
		if err != ErrInvalidVerificationToken {
			t.Errorf("expected ErrInvalidVerificationToken, got %v", err)
		}
	})

	t.Run("fails with empty cv_hash claim", func(t *testing.T) {
		claims := jwt.MapClaims{
			ClaimType:                   ClaimOauth2StateValue,
			ClaimOauth2CodeVerifierHash: "",
		}
		token, _ := NewJwt(claims, []byte(secret), duration)

		err := VerifyOauth2StateToken(token, cv, secret)
		if err != ErrInvalidClaimFormat {
			t.Errorf("expected ErrInvalidClaimFormat, got %v", err)
		}
	})
}

