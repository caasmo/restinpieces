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
            // the returned error is not passed to the parse err!!
			return nil, ErrInvalidSigningMethod
		}
		return secret, nil
	})

	if err != nil {
        if errors.Is(err, jwt.ErrTokenExpired) {
		    return nil, ErrTokenExpired
        }

        if errors.Is(err, jwt.ErrTokenUnverifiable) {
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

// Validate checks if a token is valid
func Validate(tokenString string, secret []byte) (*Claims, error) {
	claims, err := Parse(tokenString, secret)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

// Refresh creates a new token based on the claims of an existing token
func Refresh(userID string, secret []byte, tokenDuration time.Duration) (string, time.Time, error) {
	return Create(userID, secret, tokenDuration)
}
