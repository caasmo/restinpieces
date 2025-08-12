package core

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"encoding/base64"
	"regexp"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
)

// Pre-compiled regex for user_id pattern matching
// Matches: r followed by exactly 14 hex characters (lowercase)
var userIDRegex = regexp.MustCompile(`(r[0-9a-f]{14})`)

var errParseUserID = errors.New("parse user id error")

// Authenticator defines the interface for authentication operations
type Authenticator interface {
	Authenticate(r *http.Request) (*db.User, jsonResponse, error)
}

// DefaultAuthenticator implements Authenticator using the standard authentication flow
type DefaultAuthenticator struct {
	dbAuth         db.DbAuth
	logger         *slog.Logger
	configProvider *config.Provider
}

// NewDefaultAuthenticator creates a new DefaultAuthenticator instance
func NewDefaultAuthenticator(dbAuth db.DbAuth, logger *slog.Logger, configProvider *config.Provider) *DefaultAuthenticator {
	return &DefaultAuthenticator{
		dbAuth:         dbAuth,
		logger:         logger,
		configProvider: configProvider,
	}
}

// Authenticate implements the Authenticator interface
func (a *DefaultAuthenticator) Authenticate(r *http.Request) (*db.User, jsonResponse, error) {
	errAuth := errors.New("Auth error")
	// Extract token from request
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errorNoAuthHeader, errAuth
	}

	// Check for Bearer prefix
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return nil, errorInvalidTokenFormat, errAuth
	}

	// make a cheap regexp for the userId
	// before we had crypto.ParseJwtUnverified but is was almost as expensive as full verification
	userId, err := parseJwtUserID(tokenString)
	if err != nil {
		return nil, errorJwtInvalidToken, errAuth
	}

	user, err := a.dbAuth.GetUserById(userId)
	if err != nil || user == nil {
		return nil, errorJwtInvalidToken, errors.New("Auth error")
	}

	// Generate signing key using user credentials
	// Use user.Email and user.Password which are confirmed to belong to userId
	cfg := a.configProvider.Get() // Get the current config
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(user.Email, user.Password, cfg.Jwt.AuthSecret)
	if err != nil {
		// Errors here are likely config issues (e.g., short secret) or bad user data
		// Map to a generic server-side error for the client
		return nil, errorTokenGeneration, errAuth
	}

	// Verify full token signature and standard claims (like expiry)
	claims, err := crypto.ParseJwt(tokenString, signingKey)
	if err != nil {
		// Map specific JWT errors to our precomputed responses
		if errors.Is(err, crypto.ErrJwtTokenExpired) {
			return nil, errorJwtTokenExpired, errAuth
		}
		if errors.Is(err, crypto.ErrJwtInvalidSigningMethod) {
			return nil, errorJwtInvalidSignMethod, errAuth
		}
		// Treat all other verification errors as an invalid token
		return nil, errorJwtInvalidToken, errAuth
	}

	// Final validation of claims after signature is confirmed
	if err := crypto.ValidateSessionClaims(claims); err != nil {
		return nil, errorJwtInvalidToken, errAuth
	}

	// If all checks pass, return the authenticated user with empty response
	return user, jsonResponse{}, nil
}

// ParseJwtUserID extracts only the user_id from a JWT token without full verification.
// Uses regex to find the user_id pattern directly in the decoded payload.
//
// Expected user_id format: r{14 hex chars} (e.g., "r2e4d72d378c747")
func parseJwtUserID(tokenString string) (string, error) {
	// Split token into parts (header.payload.signature)
	parts := strings.SplitN(tokenString, ".", 3)
	if len(parts) != 3 {
		return "", errParseUserID
	}

	// Decode only the payload (middle part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errParseUserID
	}

	// Find user_id pattern directly: r followed by 14 hex chars
	matches := userIDRegex.FindStringSubmatch(string(payload))
	if len(matches) != 2 {
		return "", errParseUserID
	}

	return matches[1], nil
}
