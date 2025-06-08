package core

import (
	"errors"
	"net/http"
	"log/slog"
	"strings"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
)

// Authenticator defines the interface for authentication operations
type Authenticator interface {
	Authenticate(r *http.Request) (*db.User, error, jsonResponse)
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
func (a *DefaultAuthenticator) Authenticate(r *http.Request) (*db.User, error, jsonResponse) {
	errAuth := errors.New("Auth error")
	// Extract token from request
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errAuth, errorNoAuthHeader
	}

	// Check for Bearer prefix
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return nil, errAuth, errorInvalidTokenFormat
	}

	// Parse unverified token to get claims
	claims, err := crypto.ParseJwtUnverified(tokenString)
	if err != nil {
		return nil, errAuth, errorJwtInvalidToken
	}

	// Validate session claims before fetching user
	if err := crypto.ValidateSessionClaims(claims); err != nil {
		if err == crypto.ErrJwtTokenExpired {
			return nil, errAuth, errorJwtTokenExpired
		}
		return nil, errAuth, errorJwtInvalidToken
	}

	userID := claims[crypto.ClaimUserID].(string)
	user, err := a.dbAuth.GetUserById(userID)
	if err != nil || user == nil {
		return nil, errors.New("Auth error"), errorJwtInvalidToken
	}

	// Generate signing key using user credentials
	// Use user.Email and user.Password which are confirmed to belong to userID
	cfg := a.configProvider.Get() // Get the current config
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(user.Email, user.Password, cfg.Jwt.AuthSecret)
	if err != nil {
		// Errors here are likely config issues (e.g., short secret) or bad user data
		// Map to a generic server-side error for the client
		return nil, errAuth, errorTokenGeneration
	}

	// Verify token signature and standard claims (like expiry)
	_, err = crypto.ParseJwt(tokenString, signingKey)
	if err != nil {
		// Map specific JWT errors to our precomputed responses
		if err == crypto.ErrJwtTokenExpired {
			return nil, errAuth, errorJwtTokenExpired
		}
		if err == crypto.ErrJwtInvalidSigningMethod {
			return nil, errAuth, errorJwtInvalidSignMethod
		}
		// Treat all other verification errors as an invalid token
		return nil, errAuth, errorJwtInvalidToken
	}

	// If all checks pass, return the authenticated user with empty response
	return user, nil, jsonResponse{}
}
