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
			writeJsonError(w, errorNoAuthHeader)
			return
		}

		// Check for Bearer prefix
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			writeJsonError(w, errorInvalidTokenFormat)
			return
		}

		// Parse unverified token to get claims
		claims, err := crypto.ParseJwtUnverified(tokenString)
		if err != nil {
			writeJsonError(w, errorJwtInvalidToken)
			return
		}
		// Validate issued at claim
		if err := crypto.ValidateClaimIssuedAt(claims); err != nil {
			if errors.Is(err, crypto.ErrTokenTooOld) {
				writeJsonError(w, errorJwtTokenExpired)
				return
			}
			writeJsonError(w, errorJwtInvalidToken)
			return
		}
		// Validate user ID claim
		if err := crypto.ValidateClaimUserID(claims); err != nil {
			writeJsonError(w, errorJwtInvalidToken)
			return
		}
		// Get user from database
		userID := claims[crypto.ClaimUserID].(string)
		user, err := a.db.GetUserById(userID)
		if err != nil || user == nil {
			writeJsonError(w, errorJwtInvalidToken)
			return
		}
		// Generate signing key using user credentials
		signingKey, err := crypto.NewJwtSigningKeyWithCredentials(user.Email, user.Password, a.config.Jwt.AuthSecret)
		if err != nil {
			writeJsonError(w, errorTokenGeneration)
			return
		}
		// Verify token with generated signing key
		_, err = crypto.ParseJwt(tokenString, signingKey)
		if err != nil {
			if errors.Is(err, crypto.ErrJwtTokenExpired) {
				writeJsonError(w, errorJwtTokenExpired)
				return
			}
			if errors.Is(err, crypto.ErrJwtInvalidSigningMethod) {
				writeJsonError(w, errorJwtInvalidSignMethod)
				return
			}
			writeJsonError(w, errorJwtInvalidToken)
			return
		}

		// Store user ID in context
		ctx := context.WithValue(r.Context(), UserIDKey, userID)

		// Call the next handler with the new context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
