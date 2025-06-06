package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
)

// Authenticate extracts, parses, and validates a JWT token from the Authorization header.
// It returns:
// - authenticated user on success
// - error (always "Auth error" for security)
// - precomputed jsonResponse for error cases
func (a *App) Authenticate(r *http.Request) (*db.User, error, jsonResponse) {
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
		if errors.Is(err, crypto.ErrJwtTokenExpired) {
			return nil, errAuth, errorJwtTokenExpired
		}
		return nil, errAuth, errorJwtInvalidToken
	}

	userID := claims[crypto.ClaimUserID].(string)
	user, err := a.DbAuth().GetUserById(userID)
	if err != nil || user == nil {
		return nil, errors.New("Auth error"), errorJwtInvalidToken
	}

	// Generate signing key using user credentials
	// Use user.Email and user.Password which are confirmed to belong to userID
	cfg := a.Config() // Get the current config
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
		if errors.Is(err, crypto.ErrJwtTokenExpired) {
			return nil, errAuth, errorJwtTokenExpired
		}
		if errors.Is(err, crypto.ErrJwtInvalidSigningMethod) {
			return nil, errAuth, errorJwtInvalidSignMethod
		}
		// Treat all other verification errors as an invalid token
		return nil, errAuth, errorJwtInvalidToken
	}

	// If all checks pass, return the authenticated user with empty response
	return user, nil, jsonResponse{}
}
