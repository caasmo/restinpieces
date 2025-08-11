package crypto

import (
	"errors"
	"fmt"
	"time"

	"crypto/hmac"
	"crypto/sha256"

	"github.com/golang-jwt/jwt/v5"
)

// todo remove refresh and validate/create are wrappers

const (
	// MinKeyLength is the minimum required length for JWT signing keys.
	// 32 bytes (256 bits) is the minimum recommended length for HMAC-SHA256 keys
	// to provide sufficient security against brute force attacks.
	MinKeyLength = 32


	// JWT claim constants
	ClaimIssuedAt  = "iat"     // JWT Issued At claim key
	ClaimExpiresAt = "exp"     // JWT Expiration Time claim key
	ClaimUserID    = "user_id" // JWT User ID claim key

	// Email verification specific claims
	ClaimEmail              = "email"          // Email address being verified
	ClaimType               = "type"           // Verification type claim
	ClaimVerificationValue  = "verification"   // Value for verification type claim
	ClaimPasswordResetValue = "password_reset" // Value for password reset type claim
	ClaimEmailChangeValue   = "email_change"   // Value for email change type claim
	ClaimNewEmail           = "new_email"      // New email address for email change claims

	// MaxTokenAge is the maximum age a JWT token can be before it's considered too old (7 days in seconds)
	MaxTokenAge = 7 * 24 * 60 * 60
)

var (


	// ErrJwtTokenExpired is returned when the token has expired
	ErrJwtTokenExpired = errors.New("token expired")
	// ErrJwtInvalidToken is returned when the token is invalid
	ErrJwtInvalidToken = errors.New("invalid token")
	// ErrInvalidVerificationToken is returned when verification token is invalid
	ErrInvalidVerificationToken = errors.New("invalid verification token")
	// ErrJwtInvalidSigningMethod is returned when the signing method is not HS256
	ErrJwtInvalidSigningMethod = errors.New("unexpected signing method")
	// ErrJwtInvalidSecretLength is returned for invalid secret lengths
	ErrJwtInvalidSecretLength = errors.New("invalid secret length")
	// ErrInvalidSigningKeyParts is returned when email or password hash are empty
	ErrInvalidSigningKeyParts = errors.New("invalid signing key parts")
	// ErrTokenUsedBeforeIssued is returned when a token's "iat" (issued at) claim
	// is in the future, indicating the token is being used before it was issued
	ErrTokenUsedBeforeIssued = errors.New("token used before issued")
	// ErrInvalidClaimFormat is returned when a claim has the wrong type
	ErrInvalidClaimFormat = errors.New("invalid claim format")
	// ErrClaimNotFound is returned when a required claim is missing
	ErrClaimNotFound = errors.New("claim not found")
	// ErrTokenTooOld is returned when a token's "iat" (issued at) claim
	// is older than the maximum allowed age (one week)
	ErrTokenTooOld = errors.New("token too old")
)

// Implement only the validation rather than using the full validator
// but this is not lightweight either, 60% so expensive as full. 
func ParseJwtUnverified(tokenString string) (jwt.MapClaims, error) {
	claims := make(jwt.MapClaims)

	_, _, err := jwt.NewParser().ParseUnverified(tokenString, claims)
	if err != nil {
		return nil, err
	}

	return claims, nil
}


// ParseJwt verifies and parses JWT and returns its claims.
// returns a map map[string]any that you can access like any other Go map.
//
//	exp := claims["exp"].(float64)
func ParseJwt(token string, verificationKey []byte) (jwt.MapClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}))

	parsedToken, err := parser.Parse(token, func(t *jwt.Token) (any, error) {
		return verificationKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrJwtTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, ErrJwtInvalidSigningMethod
		}
		return nil, fmt.Errorf("%w: %w", ErrJwtInvalidToken, err)
	}

	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok && parsedToken.Valid {
		return claims, nil
	}

	return nil, ErrJwtInvalidToken
}

// NewJwtSession creates a new JWT session token for a user
// It handles the complete token generation process including:
// - Creating the signing key from user credentials
// - Setting up standard claims
// - Generating and signing the token
func NewJwtSessionToken(userID, email, passwordHash, secret string, duration time.Duration) (string, error) {
	// Create signing key from email and secret
	signingKey, err := NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	// Set up claims
	claims := jwt.MapClaims{
		ClaimUserID: userID,
	}

	// Generate and return token
	return NewJwt(claims, signingKey, duration)
}

// NewJwtPasswordResetToken creates a JWT specifically for password reset
func NewJwtEmailChangeToken(userID, oldEmail, newEmail, passwordHash, secret string, duration time.Duration) (string, error) {
	// Create signing key from email and secret
	signingKey, err := NewJwtSigningKeyWithCredentials(oldEmail, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	// Set up email change-specific claims
	claims := jwt.MapClaims{
		ClaimUserID:   userID,
		ClaimEmail:    oldEmail,
		ClaimNewEmail: newEmail,
		ClaimType:     ClaimEmailChangeValue,
	}

	// Generate and return token
	return NewJwt(claims, signingKey, duration)
}

func NewJwtPasswordResetToken(userID, email, passwordHash, secret string, duration time.Duration) (string, error) {
	// Create signing key from email and secret
	signingKey, err := NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	// Set up password reset-specific claims
	claims := jwt.MapClaims{
		ClaimUserID: userID,
		ClaimEmail:  email,
		ClaimType:   ClaimPasswordResetValue,
	}

	// Generate and return token
	return NewJwt(claims, signingKey, duration)
}

// NewJwtEmailVerificationToken creates a JWT specifically for email verification
// It includes additional claims needed for verification
func NewJwtEmailVerificationToken(userID, email, passwordHash, secret string, duration time.Duration) (string, error) {
	// Create signing key from email and secret
	signingKey, err := NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	// Set up verification-specific claims
	claims := jwt.MapClaims{
		ClaimUserID: userID,
		ClaimEmail:  email,
		ClaimType:   ClaimVerificationValue,
	}

	// Generate and return token
	return NewJwt(claims, signingKey, duration)
}

func NewJwt(payload jwt.MapClaims, signingKey []byte, duration time.Duration) (string, error) {
	if len(signingKey) < MinKeyLength {
		return "", ErrJwtInvalidSecretLength
	}

	// Set standard claims
	now := time.Now()
	expirationTime := now.Add(duration)
	payload[ClaimIssuedAt] = now.Unix()
	payload[ClaimExpiresAt] = expirationTime.Unix()

	// Create and sign the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	tokenString, err := token.SignedString(signingKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// NewJwtSigningKeyWithCredentials creates a JWT signing key using HMAC-SHA256.

// It derives a unique key by combining user-specific data (email, passwordHash)
// with a server secret (JWT_SECRET). Tokens are invalidated when the user's
// email or password changes, or globally by rotating JWT_SECRET.
//
// The passwordHash parameter can be empty to support passwordless authentication
// methods like OAuth2. In this case, the signing key is derived only from the
// email and server secret.
//
// Using HMAC prevents length-extension attacks, unlike simple hash concatenation.
//
// The function uses a null byte (\x00) as a delimiter to prevent collisions
// between the email and passwordHash inputs. It returns the key as a byte slice,
// suitable for use with github.com/golang-jwt/jwt/v5's SignedString method,
// and an error if the server secret is unset or inputs are invalid.
//
// Note: JWT_SECRET should be a strong, random value (e.g., 32+ bytes).
func NewJwtSigningKeyWithCredentials(email, passwordHash, secret string) ([]byte, error) {
	// Validate inputs
	if email == "" {
		return nil, ErrInvalidSigningKeyParts
	}

	// Validate server secret length
	if len(secret) < MinKeyLength {
		return nil, ErrJwtInvalidSecretLength
	}

	// Create HMAC hasher with server secret as key
	h := hmac.New(sha256.New, []byte(secret))

	// Add user-specific data, handle empty passwordHash
	h.Write([]byte(email))
	h.Write([]byte{0}) // Null byte to avoid collisions
	if passwordHash != "" {
		h.Write([]byte(passwordHash))
	}

	// Return the HMAC sum as a raw byte slice
	return h.Sum(nil), nil
}
