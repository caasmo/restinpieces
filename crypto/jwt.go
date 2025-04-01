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
	ClaimEmail             = "email"        // Email address being verified
	ClaimType              = "type"         // Verification type claim
	ClaimVerificationValue = "verification" // Value for verification type claim
	ClaimPasswordResetValue = "password_reset" // Value for password reset type claim
	ClaimEmailChangeValue   = "email_change"   // Value for email change type claim
	ClaimNewEmail          = "new_email"      // New email address for email change claims

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

// Implement only the validation you need rather than using the full validator
func ParseJwtUnverified(tokenString string) (jwt.MapClaims, error) {
	// Pre-allocate the claims map for better performance
	claims := make(jwt.MapClaims)

	_, _, err := jwt.NewParser().ParseUnverified(tokenString, claims)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

func ValidateVerificationClaims(claims jwt.MapClaims) error {

	// Validate iat claim and token age
	if err := ValidateClaimIssuedAt(claims); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate exp claim
	if err := ValidateClaimExpiresAt(claims); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate user_id claim
	if err := ValidateClaimUserID(claims); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate required claims exist
	if err := ValidateClaimEmail(claims); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	if err := ValidateClaimType(claims, ClaimVerificationValue); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	return nil
}

// TODO for verification other constant !!!!111
func ValidateClaimIssuedAt(claims jwt.MapClaims) error {
	if iat, ok := claims[ClaimIssuedAt]; ok {
		// there are two main reasons why the JWT library uses float64
		// JSON which represents all numbers as float64
		// Sub-second Precision
		if iatTime, ok := iat.(float64); ok {
			iatUnix := int64(iatTime)
			nowUnix := time.Now().Unix()
			if iatUnix > nowUnix {
				return ErrTokenUsedBeforeIssued
			}
			if nowUnix-iatUnix > MaxTokenAge {
				return ErrTokenTooOld
			}
			return nil
		}
		return ErrInvalidClaimFormat
	}
	return ErrClaimNotFound
}

// ValidateClaimUserID is a standalone function to validate the user_id claim
// that can be called separately when needed
func ValidateClaimEmail(claims jwt.MapClaims) error {
	// Check if email exists
	if email, exists := claims[ClaimEmail]; exists {
		// Verify it's a string and not empty
		if emailStr, ok := email.(string); ok {
			if emailStr == "" {
				return ErrInvalidClaimFormat
			}
			return nil
		}
		return ErrInvalidClaimFormat
	}
	return ErrClaimNotFound
}

func ValidatePasswordResetClaims(claims jwt.MapClaims) error {
	// Validate iat claim and token age
	if err := ValidateClaimIssuedAt(claims); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate exp claim
	if err := ValidateClaimExpiresAt(claims); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate user_id claim
	if err := ValidateClaimUserID(claims); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	// Validate required claims exist
	if err := ValidateClaimEmail(claims); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	if err := ValidateClaimType(claims, ClaimPasswordResetValue); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidVerificationToken, err)
	}

	return nil
}

func ValidateClaimType(claims jwt.MapClaims, value string) error {
	// Check if type claim exists
	if typeVal, exists := claims[ClaimType]; exists {
		// Verify it's a string and matches expected value
		if typeStr, ok := typeVal.(string); ok {
			if typeStr != value {
				return ErrInvalidClaimFormat
			}
			return nil
		}
		return ErrInvalidClaimFormat
	}
	return ErrClaimNotFound
}

func ValidateClaimExpiresAt(claims jwt.MapClaims) error {
	// Check if exp claim exists
	if exp, exists := claims[ClaimExpiresAt]; exists {
		// Verify it's a float64 and not expired
		if expTime, ok := exp.(float64); ok {
			now := time.Now().Unix()
			if int64(expTime) < now {
				return ErrJwtTokenExpired
			}
			return nil
		}
		return ErrInvalidClaimFormat
	}
	return ErrClaimNotFound
}

func ValidateClaimUserID(claims jwt.MapClaims) error {
	// Check if user_id exists
	if userID, exists := claims[ClaimUserID]; exists {
		// Verify it's a string and not empty
		if userIDStr, ok := userID.(string); ok {
			if userIDStr == "" {
				return ErrInvalidClaimFormat
			}
			// Additional user_id validation could go here
			return nil
		}
		return ErrInvalidClaimFormat
	}
	return ErrClaimNotFound
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
func NewJwtSessionToken(userID, email, passwordHash string, secret []byte, duration time.Duration) (string, error) {
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
func NewJwtEmailChangeToken(userID, oldEmail, newEmail, passwordHash string, secret []byte, duration time.Duration) (string, error) {
	// Create signing key from email and secret
	signingKey, err := NewJwtSigningKeyWithCredentials(oldEmail, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	// Set up email change-specific claims
	claims := jwt.MapClaims{
		ClaimUserID: userID,
		ClaimEmail:  oldEmail,
		ClaimNewEmail: newEmail,
		ClaimType:   ClaimEmailChangeValue,
	}

	// Generate and return token
	return NewJwt(claims, signingKey, duration)
}

func NewJwtPasswordResetToken(userID, email, passwordHash string, secret []byte, duration time.Duration) (string, error) {
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
func NewJwtEmailVerificationToken(userID, email, passwordHash string, secret []byte, duration time.Duration) (string, error) {
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
func NewJwtSigningKeyWithCredentials(email, passwordHash string, secret []byte) ([]byte, error) {
	// Validate inputs
	if email == "" {
		return nil, ErrInvalidSigningKeyParts
	}

	// Validate server secret length
	if len(secret) < MinKeyLength {
		return nil, ErrJwtInvalidSecretLength
	}

	// Create HMAC hasher with server secret as key
	h := hmac.New(sha256.New, secret)

	// Add user-specific data, handle empty passwordHash
	h.Write([]byte(email))
	h.Write([]byte{0}) // Null byte to avoid collisions
	if passwordHash != "" {
		h.Write([]byte(passwordHash))
	}

	// Return the HMAC sum as a raw byte slice
	return h.Sum(nil), nil
}
