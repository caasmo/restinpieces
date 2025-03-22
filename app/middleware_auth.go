package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/crypto"
)

// contextKey is a type for context keys
type contextKey string

// Context keys
// See also handler_auth.go
// TODO remove this
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

		// Parse unverified token to get claims
		claims, err := crypto.ParseJwtUnverified(tokenString)
		if err != nil {
			writeJSONError(w, errorJwtInvalidToken)
			return
		}
		// Validate issued at claim
		if err := crypto.ValidateClaimIssuedAt(claims); err != nil {
			if errors.Is(err, crypto.ErrTokenTooOld) {
				writeJSONError(w, errorJwtTokenExpired)
				return
			}
			writeJSONError(w, errorJwtInvalidToken)
			return
		}
		// Validate user ID claim
		if err := crypto.ValidateClaimUserID(claims); err != nil {
			writeJSONError(w, errorJwtInvalidToken)
			return
		}
		// Get user from database
		userID := claims[crypto.ClaimUserID].(string)
		user, err := a.db.GetUserById(userID)
		if err != nil || user == nil {
			writeJSONError(w, errorJwtInvalidToken)
			return
		}
		// Generate signing key using user credentials
		signingKey, err := crypto.NewJwtSigningKeyWithCredentials(user.Email, user.Password, a.config.JwtSecret)
		if err != nil {
			writeJSONError(w, errorTokenGeneration)
			return
		}
		// Verify token with generated signing key
		_, err = crypto.ParseJwt(tokenString, signingKey)
		if err != nil {
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

		// Store user ID in context
		ctx := context.WithValue(r.Context(), UserIDKey, userID)

		// Call the next handler with the new context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
