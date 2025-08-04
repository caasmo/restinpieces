package core

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
)

func (a *App) ConfirmEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	type request struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Validate required fields
	if req.Token == "" || req.Password == "" {
		WriteJsonError(w, errorMissingFields)
		return
	}

	// Parse unverified claims to discard fast
	claims, err := crypto.ParseJwtUnverified(req.Token)
	if err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Validate all required claims exist and have correct values
	if err := crypto.ValidateEmailChangeClaims(claims); err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	user, err := a.DbAuth().GetUserById(claims[crypto.ClaimUserID].(string))
	if err != nil || user == nil {
		WriteJsonError(w, errorNotFound)
		return
	}

	// Verify password matches current password
	if !crypto.CheckPassword(req.Password, user.Password) {
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	// Verify token signature using email change secret
	cfg := a.Config() // Get the current config
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(
		claims[crypto.ClaimEmail].(string),
		user.Password,
		cfg.Jwt.EmailChangeSecret,
	)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	// Fully verify token signature and claims
	_, err = crypto.ParseJwt(req.Token, signingKey)
	if err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Get new email from claims
	newEmail := claims["new_email"].(string)

	// Validate new email format (even though claims were validated, this is an extra check)
	if err := ValidateEmail(newEmail); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	err = a.DbAuth().UpdateEmail(user.ID, newEmail)
	if err != nil {
		WriteJsonError(w, errorServiceUnavailable)
		return
	}

	WriteJsonOk(w, okEmailChange)
}

