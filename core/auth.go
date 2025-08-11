package core

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
)

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

	// Parse unverified token to get claims
	claims := &crypto.SessionClaims{}
	_, err := crypto.ParseJwtUnverified(tokenString, claims)
	if err != nil {
		// This catches malformed tokens, but not expired ones since we don't verify yet.
		return nil, errorJwtInvalidToken, errAuth
	}

	// The 'exp' claim is validated by the final ParseJwt call.
	// The custom validation logic (like checking 'iat' and 'user_id' presence)
	// is handled by the Valid() method on the SessionClaims struct, which is
	// automatically called by ParseJwt. Therefore, an explicit call to a
	// separate validation function is no longer needed.

	userID := claims.UserID
	if userID == "" {
		// If the user ID is missing, the token is invalid. This check is technically
		// redundant if the token is later parsed with ParseJwt (which calls the Valid
		// method), but it's a good practice to fail early before a DB call.
		return nil, errorJwtInvalidToken, errAuth
	}

	user, err := a.dbAuth.GetUserById(userID)
	if err != nil || user == nil {
		return nil, errorJwtInvalidToken, errors.New("Auth error")
	}

	// Generate signing key using user credentials
	// Use user.Email and user.Password which are confirmed to belong to userID
	cfg := a.configProvider.Get() // Get the current config
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(user.Email, user.Password, cfg.Jwt.AuthSecret)
	if err != nil {
		// Errors here are likely config issues (e.g., short secret) or bad user data
		// Map to a generic server-side error for the client
		return nil, errorTokenGeneration, errAuth
	}

	// Verify token signature and all claims (including standard ones like expiry
	// and custom ones in the Valid() method).
	_, err = crypto.ParseJwt(tokenString, signingKey, &crypto.SessionClaims{})
	if err != nil {
		// Map specific JWT errors to our precomputed responses
		if err == crypto.ErrJwtTokenExpired {
			return nil, errorJwtTokenExpired, errAuth
		}
		if err == crypto.ErrJwtInvalidSigningMethod {
			return nil, errorJwtInvalidSignMethod, errAuth
		}
		// Treat all other verification errors as an invalid token
		return nil, errorJwtInvalidToken, errAuth
	}

	// If all checks pass, return the authenticated user with empty response
	return user, jsonResponse{}, nil
}
