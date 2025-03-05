package app

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)


//  export JWT_SECRET=$(openssl rand -base64 32)
//
//  First get a valid JWT token (replace JWT_SECRET with your actual secret)
//  This is a test token generation command using jwt-cli (install via 'go install github.com/matiaskorhonen/jwt-cli@latest')
//  JWT_TOKEN=$(jwt encode --secret "${JWT_SECRET}" --claim user_id=testuser123 --exp +5m)
// 
//  # Test valid token refresh
//  curl -v -X POST http://localhost:8080/auth-refresh \
//    -H "Authorization: Bearer $JWT_TOKEN"
// 
//  # Test invalid token
//  curl -v -X POST http://localhost:8080/auth-refresh \
//    -H "Authorization: Bearer invalid.token.here"
// 
//  # Test missing header
//  curl -v -X POST http://localhost:8080/auth-refresh
type jsonError struct {
	code int
	body []byte
}

var (
	jwtSecret = []byte("your_jwt_secret_here")
	jsonHeader = []string{"application/json; charset=utf-8"} // Precomputed header value
)

// Precomputed error responses with status codes
var (
	errorUnauthorized        = jsonError{http.StatusUnauthorized, []byte(`{"error":"Authorization header required"}`)}
	errorInvalidFormat       = jsonError{http.StatusUnauthorized, []byte(`{"error":"Invalid authorization format"}`)}
	errorTokenExpired        = jsonError{http.StatusUnauthorized, []byte(`{"error":"Token expired"}`)}
	errorTokenGeneration     = jsonError{http.StatusInternalServerError, []byte(`{"error":"Failed to generate token"}`)}
)

// writeJSONError writes a precomputed JSON error response
func writeJSONError(w http.ResponseWriter, err jsonError) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(err.code)
	w.Write(err.body)
}

// writeJSONErrorf writes a formatted JSON error response
func writeJSONErrorf(w http.ResponseWriter, code int, format string, args ...interface{}) {
	w.Header()["Content-Type"] = jsonHeader
	w.WriteHeader(code)
	fmt.Fprintf(w, format, args...)
}


// Custom claims structure to include standard and custom fields
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// RefreshAuthHandler handles JWT refresh requests
func (a *App) RefreshAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Extract and validate Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeJSONError(w, errorUnauthorized)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader { // No Bearer prefix found
		writeJSONError(w, errorInvalidFormat)
		return
	}

	// Parse and validate token
	claims, err := parseToken(tokenString)
	if err != nil {
		writeJSONErrorf(w, http.StatusUnauthorized, `{"error":"Invalid token: %s"}`, err.Error())
		return
	}

	// Check token expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		writeJSONError(w, errorTokenExpired)
		return
	}

	// Generate new token with extended expiration
	newToken, err := createToken(claims.UserID)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	// Return new token in response following OAuth2 token exchange format
	w.Header()["Content-Type"] = jsonHeader
	
	// Standard OAuth2 token response format
	fmt.Fprintf(w, `{
		"token_type": "Bearer",
		"expires_in": 21600,
		"access_token": "%s"
	}`, newToken)
}

// parseToken validates and parses JWT claims
func parseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}

// createToken generates a new JWT with 6-hour expiration
func createToken(userID string) (string, error) {
	expirationTime := time.Now().Add(6 * time.Hour)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

