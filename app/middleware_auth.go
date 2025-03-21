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
		slog.Debug("JWT validation started")
	
		// Extract token from request
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			slog.Debug("No Authorization header found")
			writeJSONError(w, errorNoAuthHeader)
			return
		}

		// Check for Bearer prefix
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			slog.Debug("Invalid token format - missing Bearer prefix")
			writeJSONError(w, errorInvalidTokenFormat)
			return
		}

		slog.Debug("Token extracted", "token_length", len(tokenString))

		// Parse unverified token to get claims
		slog.Debug("Parsing unverified token")
		claims, err := crypto.ParseJwtUnverified(tokenString)
		if err != nil {
			slog.Error("Failed to parse unverified token", "error", err)
			writeJSONError(w, errorJwtInvalidToken)
			return
		}
		slog.Debug("Unverified token parsed", "claims", claims)

		// Validate issued at claim
		slog.Debug("Validating issued at claim")
		if err := crypto.ValidateClaimIssuedAt(claims); err != nil {
			if errors.Is(err, crypto.ErrTokenTooOld) {
				slog.Debug("Token too old", "error", err)
				writeJSONError(w, errorJwtTokenExpired)
				return
			}
			slog.Error("Invalid issued at claim", "error", err)
			writeJSONError(w, errorJwtInvalidToken)
			return
		}
		slog.Debug("Issued at claim validated")

		// Validate user ID claim
		slog.Debug("Validating user ID claim")
		if err := crypto.ValidateClaimUserID(claims); err != nil {
			slog.Error("Invalid user ID claim", "error", err)
			writeJSONError(w, errorJwtInvalidToken)
			return
		}
		slog.Debug("User ID claim validated")

		// Get user from database
		userID := claims[crypto.ClaimUserID].(string)
		slog.Debug("Fetching user from database", "user_id", userID)
		user, err := a.db.GetUserById(userID)
		if err != nil || user == nil {
			slog.Error("Failed to fetch user", "user_id", userID, "error", err)
			writeJSONError(w, errorJwtInvalidToken)
			return
		}
		slog.Debug("User fetched", "user_id", user.ID, "email", user.Email)

		// Generate signing key using user credentials
		slog.Debug("Generating signing key", 
			"email", user.Email,
			"secret_length", len(a.config.JwtSecret))
		signingKey, err := crypto.NewJwtSigningKeyWithCredentials(user.Email, user.Password, a.config.JwtSecret)
		if err != nil {
			slog.Error("Failed to generate signing key", "error", err)
			writeJSONError(w, errorTokenGeneration)
			return
		}
		slog.Debug("Signing key generated", "key_length", len(signingKey))

		// Verify token with generated signing key
		slog.Debug("Verifying token with signing key")
		_, err = crypto.ParseJwt(tokenString, signingKey)
		if err != nil {
			if errors.Is(err, crypto.ErrJwtTokenExpired) {
				slog.Debug("Token expired", "error", err)
				writeJSONError(w, errorJwtTokenExpired)
				return
			}
			if errors.Is(err, crypto.ErrJwtInvalidSigningMethod) {
				slog.Error("Unexpected signing method", "error", err)
				writeJSONError(w, errorJwtInvalidSignMethod)
				return
			}
			slog.Error("Token verification failed", "error", err)
			writeJSONErrorf(w, http.StatusUnauthorized, `{"error":"Invalid token: %s"}`, err.Error())
			return
		}
		slog.Debug("Token verified successfully")

		// Store user ID in context
		ctx := context.WithValue(r.Context(), UserIDKey, userID)

		// Call the next handler with the new context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
