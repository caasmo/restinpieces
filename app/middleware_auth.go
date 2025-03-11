package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/crypto"
)

// contextKey is a type for context keys
type contextKey string

// Context keys
// See also handler_auth.go
const (
	UserIDKey contextKey = "user_id"
)

// JwtValidate middleware validates the JWT token
func (a *App) JwtValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Extract token from request
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSONError(w, errorNoAuthHeader)
			return
		}

		// Check for Bearer prefix
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			writeJSONError(w, errorInvalidTokenFormat)
			return
		}

		// Parse and validate the token
		claims, err := crypto.ParseJwt(tokenString, a.config.JwtSecret)
		if err != nil {
			// some common errors
			if errors.Is(err, crypto.ErrJwtTokenExpired) {
				writeJSONError(w, errorJwtTokenExpired)
				return
			}

			if errors.Is(err, crypto.ErrJwtInvalidSigningMethod) {
				writeJSONError(w, errorJwtInvalidSignMethod)
				return
			}

			writeJSONErrorf(w, http.StatusUnauthorized, `{"error":"Invalid token: %s"}`, err.Error())
			return
		}

		// Store claims in context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)

		// Call the next handler with the new context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
