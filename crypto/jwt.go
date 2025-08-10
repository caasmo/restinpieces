package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// MinKeyLength is the minimum required length for JWT signing keys.
	// 32 bytes (256 bits) is the minimum recommended length for HMAC-SHA256 keys
	// to provide sufficient security against brute force attacks.
	MinKeyLength = 32

	// ClaimVerificationValue is the value for the "type" claim for email verification tokens.
	ClaimVerificationValue = "verification"
	// ClaimPasswordResetValue is the value for the "type" claim for password reset tokens.
	ClaimPasswordResetValue = "password_reset"
	// ClaimEmailChangeValue is the value for the "type" claim for email change tokens.
	ClaimEmailChangeValue = "email_change"

	// MaxTokenAge is the maximum age a JWT token can be before it's considered too old.
	MaxTokenAge = 7 * 24 * time.Hour
)

var (
	// ErrJwtTokenExpired is returned when the token has expired.
	ErrJwtTokenExpired = errors.New("token expired")
	// ErrJwtInvalidToken is returned when the token is invalid.
	ErrJwtInvalidToken = errors.New("invalid token")
	// ErrInvalidVerificationToken is returned when a verification token is invalid.
	ErrInvalidVerificationToken = errors.New("invalid verification token")
	// ErrJwtInvalidSigningMethod is returned when the signing method is not HS256.
	ErrJwtInvalidSigningMethod = errors.New("unexpected signing method")
	// ErrJwtInvalidSecretLength is returned for invalid secret lengths.
	ErrJwtInvalidSecretLength = errors.New("invalid secret length")
	// ErrInvalidSigningKeyParts is returned when email or password hash are empty.
	ErrInvalidSigningKeyParts = errors.New("invalid signing key parts")
	// ErrTokenUsedBeforeIssued is returned when a token's "iat" claim is in the future.
	ErrTokenUsedBeforeIssued = errors.New("token used before issued")
	// ErrInvalidClaimFormat is returned when a claim has the wrong type or format.
	ErrInvalidClaimFormat = errors.New("invalid claim format")
	// ErrClaimNotFound is returned when a required claim is missing.
	ErrClaimNotFound = errors.New("claim not found")
	// ErrTokenTooOld is returned when a token's "iat" claim is older than MaxTokenAge.
	ErrTokenTooOld = errors.New("token too old")
)

// SessionClaims defines the claims for a standard user session token.
type SessionClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// VerificationClaims defines the claims for email verification or password reset tokens.
type VerificationClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

// EmailChangeClaims defines the claims for changing a user's email address.
type EmailChangeClaims struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	NewEmail string `json:"new_email"`
	Type     string `json:"type"`
	jwt.RegisteredClaims
}

// translateJWTError converts errors from the jwt library into application-specific errors.
func translateJWTError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, jwt.ErrTokenExpired):
		return ErrJwtTokenExpired
	case errors.Is(err, jwt.ErrTokenUsedBeforeIssued):
		return ErrTokenUsedBeforeIssued
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return ErrTokenUsedBeforeIssued // Treat "not valid yet" as "used before issued"
	case errors.Is(err, jwt.ErrTokenSignatureInvalid):
		return ErrJwtInvalidSigningMethod
	// This catches a broad range of validation errors (e.g., malformed, invalid claims)
	case errors.Is(err, jwt.ErrTokenInvalidClaims):
		return ErrInvalidClaimFormat
	default:
		// Check for specific validation errors wrapped in the main error
		if expired := new(jwt.ErrTokenExpired); errors.As(err, &expired) {
			return ErrJwtTokenExpired
		}
		// Return a generic invalid token error for all other cases.
		return fmt.Errorf("%w: %v", ErrJwtInvalidToken, err)
	}
}

// ParseJwt verifies and parses a JWT string into the provided claims struct.
// It uses generics to allow parsing into any struct that satisfies jwt.Claims.
func ParseJwt[T jwt.Claims](tokenString string, verificationKey []byte, claims T) (T, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)

	_, err := parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		// Ensure the token's signing method is what we expect.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", ErrJwtInvalidSigningMethod, t.Header["alg"])
		}
		return verificationKey, nil
	})

	if err != nil {
		return claims, translateJWTError(err)
	}

	return claims, nil
}

// ParseJwtUnverified parses a JWT without verifying its signature.
// NOTE: This should only be used when the token's authenticity is already trusted or not required.
func ParseJwtUnverified[T jwt.Claims](tokenString string, claims T) (T, error) {
	_, _, err := jwt.NewParser().ParseUnverified(tokenString, claims)
	if err != nil {
		return claims, translateJWTError(err)
	}
	return claims, nil
}

// NewJwtSessionToken creates a new JWT session token for a user.
func NewJwtSessionToken(userID, email, passwordHash, secret string, duration time.Duration) (string, error) {
	signingKey, err := NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	now := time.Now()
	claims := SessionClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// NewJwtEmailChangeToken creates a JWT for changing a user's email.
func NewJwtEmailChangeToken(userID, oldEmail, newEmail, passwordHash, secret string, duration time.Duration) (string, error) {
	signingKey, err := NewJwtSigningKeyWithCredentials(oldEmail, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	now := time.Now()
	claims := EmailChangeClaims{
		UserID:   userID,
		Email:    oldEmail,
		NewEmail: newEmail,
		Type:     ClaimEmailChangeValue,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// NewJwtPasswordResetToken creates a JWT for password reset.
func NewJwtPasswordResetToken(userID, email, passwordHash, secret string, duration time.Duration) (string, error) {
	signingKey, err := NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	now := time.Now()
	claims := VerificationClaims{
		UserID: userID,
		Email:  email,
		Type:   ClaimPasswordResetValue,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// NewJwtEmailVerificationToken creates a JWT for email verification.
func NewJwtEmailVerificationToken(userID, email, passwordHash, secret string, duration time.Duration) (string, error) {
	signingKey, err := NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	now := time.Now()
	claims := VerificationClaims{
		UserID: userID,
		Email:  email,
		Type:   ClaimVerificationValue,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// NewJwtSigningKeyWithCredentials creates a JWT signing key using HMAC-SHA256.
// It derives a unique key by combining user-specific data (email, passwordHash)
// with a server secret (JWT_SECRET).
func NewJwtSigningKeyWithCredentials(email, passwordHash, secret string) ([]byte, error) {
	if email == "" {
		return nil, ErrInvalidSigningKeyParts
	}
	if len(secret) < MinKeyLength {
		return nil, ErrJwtInvalidSecretLength
	}

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(email))
	h.Write([]byte{0}) // Null byte delimiter
	if passwordHash != "" {
		h.Write([]byte(passwordHash))
	}
	return h.Sum(nil), nil
}