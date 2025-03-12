package crypto

import (
	"errors"
	"fmt"
	"time"

    "crypto/hmac"
    "crypto/sha256"
    "os"

	"github.com/golang-jwt/jwt/v5"
)

// todo remove refresh and validate/create are wrappers

var (
	// ErrJwtTokenExpired is returned when the token has expired
	ErrJwtTokenExpired = errors.New("token expired")
	// ErrJwtInvalidToken is returned when the token is invalid
	ErrJwtInvalidToken = errors.New("invalid token")
	// ErrJwtInvalidSigningMethod is returned when the signing method is not HMAC
	ErrJwtInvalidSigningMethod = errors.New("unexpected signing method")
	// ErrJwtInvalidSecretLength is returned for invalid secret lengths
	ErrJwtInvalidSecretLength = errors.New("invalid secret length")
)

// Claims extends standard JWT claims with custom fields
// TODO not needed
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// ParseJwt verifies and parses JWT and returns its claims.
// returns a map map[string]interface{} that you can access like any other Go map. 
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

// CreateJwt generates a new JWT token
func CreateJwt(userID string, secret []byte, tokenDuration time.Duration) (string, time.Time, error) {
	if len(secret) < 32 {
		return "", time.Time{}, ErrJwtInvalidSecretLength
	}

	expirationTime := time.Now().Add(tokenDuration)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, expirationTime, nil
}


// createJwtSigningKey creates a JWT signing key using HMAC-SHA256.
//
// Using HMAC prevents length-extension attacks, unlike simple hash concatenation.
// It derives a unique key by combining user-specific data (email, passwordHash)
// with a server secret (JWT_SECRET). Tokens are invalidated when the user's
// email or password changes, or globally by rotating JWT_SECRET.
// 
// The function uses a null byte (\x00) as a delimiter to prevent collisions
// between the email and passwordHash inputs. It returns the key as a byte slice,
// suitable for use with github.com/golang-jwt/jwt/v5's SignedString method,
// and an error if the server secret is unset or inputs are invalid.
// 
// Note: JWT_SECRET should be a strong, random value (e.g., 32+ bytes).
func createJwtSigningKey(email, passwordHash string) ([]byte, error) {
    // Validate inputs
    if email == "" || passwordHash == "" {
        return nil, errors.New("email and passwordHash must not be empty")
    }

    // Retrieve the server secret
// TODO to signature
    secret := os.Getenv("JWT_SECRET")
    if secret == "" {
        return nil, errors.New("JWT_SECRET environment variable not set")
    }

    // Create HMAC hasher with server secret as key
    h := hmac.New(sha256.New, []byte(secret))

    // Add user-specific data with null byte delimiter
    h.Write([]byte(email))
    h.Write([]byte{0}) // Null byte to avoid collisions
    h.Write([]byte(passwordHash))

    // Return the HMAC sum as a raw byte slice
    return h.Sum(nil), nil
}

