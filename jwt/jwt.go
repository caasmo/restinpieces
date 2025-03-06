package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// todo remove refresh and validate/create are wrappers

var (
	// ErrTokenExpired is returned when the token has expired
	ErrTokenExpired = errors.New("token expired")
	// ErrInvalidToken is returned when the token is invalid
	ErrInvalidToken = errors.New("invalid token")
	// ErrInvalidSigningMethod is returned when the signing method is not HMAC
	ErrInvalidSigningMethod = errors.New("unexpected signing method")
	// ErrBadSecretLength is returned for invalid secret lengths
	ErrInvalidSecretLength = errors.New("invalid secret length")
)

// Claims extends standard JWT claims with custom fields
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// Parse validates and parses JWT claims
func Parse(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            // jwt package wraps this error with jwt.ErrTokenUnverifiable 
			return nil, ErrInvalidSigningMethod
		}
		return secret, nil
	})

	if err != nil {

        // Common errors
        if errors.Is(err, jwt.ErrTokenExpired) {
		    return nil, ErrTokenExpired
        }

        // we need to check here with Is instead of == because error wrapped. 
        if errors.Is(err, ErrInvalidSigningMethod) {
            return nil, ErrInvalidSigningMethod
        }
        
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// Create generates a new JWT token
func Create(userID string, secret []byte, tokenDuration time.Duration) (string, time.Time, error) {
	if len(secret) == 0 || len(secret) < 32 {
		return "", time.Time{}, ErrInvalidSecretLength 
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

