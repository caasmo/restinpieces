package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

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
	// Stateless cryptographic proof of the user_id
	ClaimUidMac    = "uid_mac" // JWT User ID MAC claim key

	// Email verification specific claims
	ClaimEmail              = "email"          // Email address being verified
	ClaimType               = "type"           // Verification type claim
	ClaimVerificationValue  = "verification"   // Value for verification type claim
	ClaimPasswordResetValue = "password_reset" // Value for password reset type claim
	ClaimEmailChangeValue   = "email_change"   // Value for email change type claim
	ClaimNewEmail           = "new_email"      // New email address for email change claims

	// OTP verification specific claims
	ClaimEmailOtpVerificationHash  = "otp_hash" // SHA256 hash of the OTP code
	ClaimEmailOtpVerificationValue = "otp"      // Value for OTP verification type claim

	// MaxTokenAge is the maximum age a JWT token can be before it's considered too old (7 days in seconds)
	MaxTokenAge = 7 * 24 * 60 * 60
)

var (
	// ErrJwtTokenExpired is returned when the token has expired
	ErrJwtTokenExpired = errors.New("token expired")
	// ErrJwtInvalidToken is returned when the token is invalid
	ErrJwtInvalidToken = errors.New("invalid token")
	// ErrInvalidEmailOtpVerificationToken is returned when email OTP verification token is invalid
	ErrInvalidEmailOtpVerificationToken = errors.New("invalid email otp token")
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



// ====================================================================================
// JWT PARSING & VALIDATION
// ====================================================================================

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

// ====================================================================================
// JWT GENERATION
// ====================================================================================

// NewJwtSession creates a new JWT session token for a user
func NewJwtSessionToken(userID, email, passwordHash, secret string, duration time.Duration) (string, error) {
	signingKey, err := NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	// Set up claims AND inject the cryptographic MAC
	claims := jwt.MapClaims{
		ClaimUserID: userID,
		ClaimUidMac: GenerateUserMac(userID, secret),
	}

	return NewJwt(claims, signingKey, duration)
}

// NewJwtEmailChangeToken creates a JWT specifically for email change
func NewJwtEmailChangeToken(userID, oldEmail, newEmail, passwordHash, secret string, duration time.Duration) (string, error) {
	signingKey, err := NewJwtSigningKeyWithCredentials(oldEmail, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	claims := jwt.MapClaims{
		ClaimUserID:   userID,
		ClaimUidMac:   GenerateUserMac(userID, secret), // Inject MAC
		ClaimEmail:    oldEmail,
		ClaimNewEmail: newEmail,
		ClaimType:     ClaimEmailChangeValue,
	}

	return NewJwt(claims, signingKey, duration)
}

// NewJwtPasswordResetToken creates a JWT specifically for password reset
func NewJwtPasswordResetToken(userID, email, passwordHash, secret string, duration time.Duration) (string, error) {
	signingKey, err := NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	claims := jwt.MapClaims{
		ClaimUserID: userID,
		ClaimUidMac: GenerateUserMac(userID, secret), // Inject MAC
		ClaimEmail:  email,
		ClaimType:   ClaimPasswordResetValue,
	}

	return NewJwt(claims, signingKey, duration)
}

// NewJwtEmailVerificationToken creates a JWT specifically for email verification
func NewJwtEmailVerificationToken(userID, email, passwordHash, secret string, duration time.Duration) (string, error) {
	signingKey, err := NewJwtSigningKeyWithCredentials(email, passwordHash, secret)
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	claims := jwt.MapClaims{
		ClaimUserID: userID,
		ClaimUidMac: GenerateUserMac(userID, secret), // Inject MAC
		ClaimEmail:  email,
		ClaimType:   ClaimVerificationValue,
	}

	return NewJwt(claims, signingKey, duration)
}

func NewJwtEmailOtpVerificationToken(email, secret string, duration time.Duration) (otp string, token string, err error) {
	if len(secret) < MinKeyLength {
		return "", "", ErrJwtInvalidSecretLength
	}

	otp = RandomNumericOTP()
	otpHash := HashOtp(otp, secret)

	// Note: OTP tokens do not contain a UserID, so we do not generate a UidMac here.
	claims := jwt.MapClaims{
		ClaimEmail:                    email,
		ClaimEmailOtpVerificationHash: otpHash,
		ClaimType:                     ClaimEmailOtpVerificationValue,
	}

	token, err = NewJwt(claims, []byte(secret), duration)
	if err != nil {
		return "", "", err
	}

	return otp, token, nil
}

func VerifyEmailOtpVerificationToken(userOtp, tokenString, secret string) (string, error) {
	if len(secret) < MinKeyLength {
		return "", ErrJwtInvalidSecretLength
	}

	claims, err := ParseJwt(tokenString, []byte(secret))
	if err != nil {
		return "", err
	}

	if err := ValidateEmailOtpVerificationClaims(claims); err != nil {
		return "", err
	}

	expectedHash, _ := claims[ClaimEmailOtpVerificationHash].(string)
	userHash := HashOtp(userOtp, secret)

	if !hmac.Equal([]byte(userHash), []byte(expectedHash)) {
		return "", ErrInvalidEmailOtpVerificationToken
	}

	email, _ := claims[ClaimEmail].(string)
	return email, nil
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
