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
		// Map generic parse error to our specific error
		return nil, errAuth, errorJwtInvalidToken
	}

	// Validate essential claims before fetching user
	if err := crypto.ValidateClaimIssuedAt(claims); err != nil {
		if errors.Is(err, crypto.ErrTokenUsedBeforeIssued) {
			// Although unlikely for session tokens, handle just in case
			return nil, errAuth, errorJwtInvalidToken
		}
		// Map other potential 'iat' errors if needed, otherwise treat as invalid
		return nil, errAuth, errorJwtInvalidToken
	}
	if err := crypto.ValidateClaimUserID(claims); err != nil {
		return nil, errAuth, errorJwtInvalidToken
	}
	// We don't validate expiry here yet, as ParseJwt will do it after signature verification.

	// Get user from database using UserID from claims
	userID := claims[crypto.ClaimUserID].(string)
	user, err := a.db.GetUserById(userID)
	// Important: Check for both error and nil user, as GetUserById might return (nil, nil) if not found
	if err != nil || user == nil {
		// Treat DB errors or user not found as invalid token scenario for security
		return nil, errors.New("Auth error"), errorJwtInvalidToken
	}

	// Generate signing key using user credentials
	// Use user.Email and user.Password which are confirmed to belong to userID
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(user.Email, user.Password, a.config.Jwt.AuthSecret)
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
