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
)

func ParseUnverifiedJWT(tokenString string) (jwt.MapClaims, error) {
    // Pre-allocate the claims map for better performance
    claims := make(jwt.MapClaims)

    _, _, err := jwt.NewParser().ParseUnverified(tokenString, claims)
    if err != nil {
        return nil, err
    }
}

// like the validator above
//If the token has an "iat" claim
//If the "iat" time is valid (properly formatted as a numeric value)
//Most importantly, it verifies that the "iat" time is not in the future (meaning the token hasn't been used before it was issued)
// for ath jwt, validate id
// If you need validation of specific claims, do it selectively
	// rather than using the full validator
//	if iat, ok := claims["iat"]; ok {
//		if iatTime, ok := iat.(float64); ok {
//			if time.Unix(int64(iatTime), 0).After(time.Now()) {
//				return nil, jwt.ErrTokenUsedBeforeIssued
//			}
//		}
//	}

// validate userId
// Validate user_id exists and is a non-empty string
	//if userID, ok := claims["user_id"]; ok {
	//	if userIDStr, ok := userID.(string); ok {
	//		if userIDStr == "" {
	//			return nil, fmt.Errorf("user_id claim is empty")
	//		}
	//		// Additional validation for user_id format could be added here
	//	} else {
	//		return nil, fmt.Errorf("user_id claim is not a string")
	//	}
	//} else {
	//	return nil, fmt.Errorf("user_id claim is required but not found")
	//}



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

