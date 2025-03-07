package crypto

import (
	"errors"
	"fmt"
	"time"

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
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// ParseJwt validates and parses JWT claims
func ParseJwt(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrJwtInvalidSigningMethod
		}
		return secret, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrJwtTokenExpired
		}
		if errors.Is(err, ErrJwtInvalidSigningMethod) {
			return nil, ErrJwtInvalidSigningMethod
		}
		return nil, fmt.Errorf("%w: %w", ErrJwtInvalidToken, err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrInvalidToken
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
