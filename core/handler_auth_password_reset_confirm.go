package core

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
)

// ConfirmPasswordResetHandler handles password reset confirmation
// Endpoint: POST /confirm-password-reset
// Authenticated: No
// Allowed Mimetype: application/json
func (a *App) ConfirmPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	type request struct {
		Token           string `json:"token"`
		Password        string `json:"password"`
		PasswordConfirm string `json:"password_confirm"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Validate required fields
	if req.Token == "" || req.Password == "" || req.PasswordConfirm == "" {
		WriteJsonError(w, errorMissingFields)
		return
	}

	// Validate password match
	if req.Password != req.PasswordConfirm {
		WriteJsonError(w, errorPasswordMismatch)
		return
	}

	// Validate password complexity
	if len(req.Password) < 8 {
		WriteJsonError(w, errorPasswordComplexity)
		return
	}

	// Parse unverified claims to discard fast
	claims, err := crypto.ParseJwtUnverified(req.Token)
	if err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Validate all required claims exist and have correct values
	// TODO Validate methods more to request Jwt. no crypto
	if err := crypto.ValidatePasswordResetClaims(claims); err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Get user from database to get password hash for signing key
	user, err := a.DbAuth().GetUserById(claims[crypto.ClaimUserID].(string))
	if err != nil || user == nil {
		WriteJsonError(w, errorNotFound)
		return
	}

	// Verify token signature using password reset secret
	cfg := a.Config() // Get the current config
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(
		claims[crypto.ClaimEmail].(string),
		user.Password,
		cfg.Jwt.PasswordResetSecret,
	)
	if err != nil {
		WriteJsonError(w, errorPasswordResetFailed)
		return
	}

	// Fully verify token signature and claims
	_, err = crypto.ParseJwt(req.Token, signingKey)
	if err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Hash new password before storage
	hashedPassword, err := crypto.GenerateHash(req.Password)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	// Check if new password matches old one
	if crypto.CheckPassword(req.Password, user.Password) {
		WriteJsonOk(w, okPasswordResetNotNeeded)
		return
	}

	// Update user password
	err = a.DbAuth().UpdatePassword(user.ID, string(hashedPassword))
	if err != nil {
		WriteJsonError(w, errorServiceUnavailable)
		return
	}
	WriteJsonOk(w, okPasswordReset)
}

