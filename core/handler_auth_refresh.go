package core

import (
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
)

// RefreshAuthHandler handles explicit JWT token refresh requests
// Endpoint: POST /auth-refresh
// Authenticated: Yes
// Allowed Mimetype: application/json
func (a *App) RefreshAuthHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}
	// Authenticate the user using the token from the request
	user, authResp, err := a.Auth().Authenticate(r)
	if err != nil {
		WriteJsonError(w, authResp)
		return
	}

	// If authentication is successful, 'user' is the authenticated user object.
	// No need to fetch the user again.

	// Generate new token with fresh expiration using NewJwtSession
	cfg := a.Config() // Get the current config
	newToken, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, cfg.Jwt.AuthSecret, cfg.Jwt.AuthTokenDuration.Duration)
	if err != nil {
		a.Logger().Error("Failed to generate new token", "error", err)
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication response
	writeAuthResponse(w, newToken, user)

}

