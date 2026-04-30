package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Defined in RFC 7636 (PKCE). Allowed characters: A-Z, a-z, 0-9, and the symbols -, ., _, ~.
const pkceAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"

// PKCE code challenge method as defined in RFC 7636
const PKCECodeChallengeMethod = "S256"

// The OAuth2 specification (RFC 6749) doesn’t mandate a specific length. It
// recommends a random, unguessable string.
// At least 16 characters, though 32 to 64 characters is common
// for better uniqueness and security.
const Oauth2StateLength = 32

// Defined in RFC 7636 (PKCE). Its length must be between 43 and 128 characters.
const OauthCodeVerifierLength = 43

const ClaimOauth2CodeVerifierHash = "cv_hash"
const ClaimOauth2StateValue = "oauth2_state"

var (
	// ErrInvalidCodeVerifier is returned when a PKCE code_verifier is malformed.
	ErrInvalidCodeVerifier = errors.New("invalid code verifier")
)

// The state parameter helps prevent Cross-Site Request Forgery (CSRF) attacks
// by linking the authorization request to its callback.
// Should be URL-safe, Here alphanumeric characters.
func Oauth2State() string {
	return RandomString(Oauth2StateLength, AlphanumericAlphabet)
}
func Oauth2CodeVerifier() string {
	return RandomString(OauthCodeVerifierLength, pkceAlphabet)
}

// S256Challenge creates base64 encoded sha256 challenge string derived from code.
// The padding of the result base64 string is stripped per [RFC 7636].
//
// [RFC 7636]: https://datatracker.ietf.org/doc/html/rfc7636#section-4.2
func S256Challenge(code string) string {
	h := sha256.New()
	h.Write([]byte(code))
	return strings.TrimRight(base64.URLEncoding.EncodeToString(h.Sum(nil)), "=")
}

// ValidateCodeVerifier reports whether s is a well-formed PKCE code_verifier
// as defined by RFC 7636 §4.1: 43–128 characters from the PKCE alphabet.
// ValidateCodeVerifier checks s is a well-formed PKCE code_verifier per RFC 7636 §4.1.
func ValidateCodeVerifier(s string) error {
	if len(s) != OauthCodeVerifierLength {
		return fmt.Errorf("%w: invalid length %d, expected %d", ErrInvalidCodeVerifier, len(s), OauthCodeVerifierLength)
	}
	for _, c := range s {
		if !strings.ContainsRune(pkceAlphabet, c) {
			return fmt.Errorf("%w: invalid character %q", ErrInvalidCodeVerifier, c)
		}
	}
	return nil
}

// HashOauth2CodeVerifier creates an HMAC-SHA256 hash of the PKCE code_verifier.
// We use HMAC with the server's secret instead of a plain SHA256 to prevent
// length-extension attacks and to ensure the hash cannot be computed offline
// without the server's key.
func HashOauth2CodeVerifier(cv, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(cv))
	return hex.EncodeToString(h.Sum(nil))
}

// NewJwtOauth2StateToken creates a stateless JWT state token that cryptographically binds
// the authorization flow to a specific code_verifier generated for the client.
//
// # Architecture Details & Purpose
// The state token serves as an absolute protection mechanism against Confused Deputy
// (Outbound DoS) and Login CSRF attacks.
//
// Even if the OAuth Provider does not support PKCE, we unconditionally use the
// code_verifier as a high-entropy (43-128 chars) client-side nonce. By embedding its
// hash inside this signed JWT, our backend ensures that any incoming /auth-with-oauth2
// request originated from the exact same client session that requested the providers list.
//
// This approach is 100% stateless (no database hits required) and IP-agnostic.
func NewJwtOauth2StateToken(codeVerifier, secret string, duration time.Duration) (string, error) {
	if len(secret) < MinKeyLength {
		return "", ErrJwtInvalidSecretLength
	}

	cvHash := HashOauth2CodeVerifier(codeVerifier, secret)

	claims := jwt.MapClaims{
		ClaimOauth2CodeVerifierHash: cvHash,
		ClaimType:                   ClaimOauth2StateValue,
	}

	return NewJwt(claims, []byte(secret), duration)
}

// VerifyOauth2StateToken parses the JWT state token and verifies that its embedded
// code_verifier hash perfectly matches the hash of the provided code_verifier.
// It fails if the token is expired, tampered with, or if the code_verifier is mismatched.
func VerifyOauth2StateToken(tokenString, codeVerifier, secret string) error {
	if len(secret) < MinKeyLength {
		return ErrJwtInvalidSecretLength
	}

	claims, err := ParseJwt(tokenString, []byte(secret))
	if err != nil {
		return err
	}

	// Verify the token type to prevent token confusion
	if typ, ok := claims[ClaimType].(string); !ok || typ != ClaimOauth2StateValue {
		return ErrInvalidVerificationToken
	}

	// Extract the embedded hash
	expectedHash, ok := claims[ClaimOauth2CodeVerifierHash].(string)
	if !ok || expectedHash == "" {
		return ErrInvalidClaimFormat
	}

	// Hash the incoming code_verifier and verify it matches the signed expectation
	userHash := HashOauth2CodeVerifier(codeVerifier, secret)
	if !hmac.Equal([]byte(userHash), []byte(expectedHash)) {
		return errors.New("invalid oauth2 state token: cv mismatch")
	}

	return nil
}
