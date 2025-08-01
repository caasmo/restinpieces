package core

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
)

// AuthWithPasswordHandler handles password-based authentication (login)
// Endpoint: POST /auth-with-password
// Authenticated: No
// Allowed Mimetype: application/json
func (a *App) AuthWithPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	var req struct {
		Identity string `json:"identity"` // username or email, only mail implemented
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if req.Identity == "" || req.Password == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Validate email format
	if err := ValidateEmail(req.Identity); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	user, err := a.DbAuth().GetUserByEmail(req.Identity)
	if err != nil || user == nil {
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	// Verify password hash
	if !crypto.CheckPassword(req.Password, user.Password) {
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	// Generate JWT session token
	cfg := a.Config() // Get the current config
	token, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, cfg.Jwt.AuthSecret, cfg.Jwt.AuthTokenDuration.Duration)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication response
	writeAuthResponse(w, token, user)
}
