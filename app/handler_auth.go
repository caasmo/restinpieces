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
var (
	jwtSecret = []byte("your_jwt_secret_here")
	jsonHeader = []string{"application/json; charset=utf-8"} // Precomputed header value
	// Precomputed error responses with status codes
	errorUnauthorized        = struct{code int; body []byte}{http.StatusUnauthorized, []byte(`{"error":"Authorization header required"}`)}
	errorInvalidFormat       = struct{code int; body []byte}{http.StatusUnauthorized, []byte(`{"error":"Invalid authorization format"}`)}
	errorTokenExpired        = struct{code int; body []byte}{http.StatusUnauthorized, []byte(`{"error":"Token expired"}`)}
	errorTokenGeneration     = struct{code int; body []byte}{http.StatusInternalServerError, []byte(`{"error":"Failed to generate token"}`)}
)

// writeError handles all error responses with precomputed values
func (a *App) writeError(w http.ResponseWriter, e struct{code int; body []byte}) {
	h := w.Header()
	h["Content-Type"] = jsonHeader
	w.WriteHeader(e.code)
	w.Write(e.body)
}

// writeDynamicError handles errors with variable messages
func (a *App) writeDynamicError(w http.ResponseWriter, code int, format string, args ...any) {
	h := w.Header()
	h["Content-Type"] = jsonHeader
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
		a.writeError(w, errorUnauthorized)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader { // No Bearer prefix found
		a.writeError(w, errorInvalidFormat)
		return
	}

	// Parse and validate token
	claims, err := parseToken(tokenString)
	if err != nil {
		a.writeDynamicError(w, http.StatusUnauthorized, `{"error":"Invalid token: %s"}`, err.Error())
		return
	}

	// Check token expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		a.writeError(w, errorTokenExpired)
		return
	}

	// Generate new token with extended expiration
	newToken, err := createToken(claims.UserID)
	if err != nil {
		a.writeError(w, errorTokenGeneration)
		return
	}

	// Return new token in response
	h := w.Header()
	h["Authorization"] = []string{"Bearer " + newToken}
	h["Content-Type"] = jsonHeader
	fmt.Fprintf(w, `{"status":"token refreshed","token":"%s"}`, newToken)
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

