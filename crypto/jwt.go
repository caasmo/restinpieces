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
)

var (
	// ErrJwtTokenExpired is returned when the token has expired
	ErrJwtTokenExpired = errors.New("token expired")
	// ErrJwtInvalidToken is returned when the token is invalid
	ErrJwtInvalidToken = errors.New("invalid token")
	// ErrJwtInvalidSigningMethod is returned when the signing method is not HS256
	ErrJwtInvalidSigningMethod = errors.New("unexpected signing method")
	// ErrJwtInvalidSecretLength is returned for invalid secret lengths
	ErrJwtInvalidSecretLength = errors.New("invalid secret length")
	// ErrTokenUsedBeforeIssued is returned when a token's "iat" (issued at) claim
	// is in the future, indicating the token is being used before it was issued
	ErrTokenUsedBeforeIssued = errors.New("token used before issued")
	// ErrInvalidClaimFormat is returned when a claim has the wrong type
	ErrInvalidClaimFormat = errors.New("invalid claim format")
	// ErrClaimNotFound is returned when a required claim is missing
	ErrClaimNotFound = errors.New("claim not found")
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

func ValidateIssuedAt(claims jwt.MapClaims) error {
	if iat, ok := claims["iat"]; ok {
		if iatTime, ok := iat.(float64); ok {
			if int64(iatTime) > time.Now().Unix() {
				return ErrTokenUsedBeforeIssued
			}
			return nil
		}
		return ErrInvalidClaimFormat
	}
	return ErrClaimNotFound
}

// ValidateUserID is a standalone function to validate the user_id claim
// that can be called separately when needed
func ValidateUserID(claims jwt.MapClaims) error {
	// Check if user_id exists
	if userID, exists := claims["user_id"]; exists {
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
// 		 exp := claims["exp"].(float64)
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





// NewJWT generates a new JWT token with the provided claims
// payload is jwt.MapClaims which is just map[string]any
// you can just call payload := map[string]any{"user_id": userID}
func NewJwt(payload jwt.MapClaims, signingKey []byte, duration time.Duration) (string, time.Time, error) {
	if len(signingKey) < MinKeyLength {
		return "", time.Time{}, ErrJwtInvalidSecretLength
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
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expirationTime, nil
}


// NewJwtSigningKeyWithCredentials creates a JWT signing key using HMAC-SHA256.

// It derives a unique key by combining user-specific data (email, passwordHash)
// with a server secret (JWT_SECRET). Tokens are invalidated when the user's
// email or password changes, or globally by rotating JWT_SECRET.
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
    if email == "" || passwordHash == "" {
        return nil, ErrJwtInvalidSecretLength 
    }

    // Validate server secret length
    if len(secret) < MinKeyLength {
        return nil, ErrJwtInvalidSecretLength
    }

    // Create HMAC hasher with server secret as key
    h := hmac.New(sha256.New, secret)

    // Add user-specific data with null byte delimiter
    h.Write([]byte(email))
    h.Write([]byte{0}) // Null byte to avoid collisions
    h.Write([]byte(passwordHash))

    // Return the HMAC sum as a raw byte slice
    return h.Sum(nil), nil
}

