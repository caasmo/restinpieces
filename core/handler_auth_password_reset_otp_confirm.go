package core

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
)

// ConfirmPasswordResetOtpHandler handles the final step of password reset.
// Endpoint: POST /confirm-password-reset-otp
// Authenticated: No
// Allowed Mimetype: application/json
//
// # Flow
//
// The user submits their new password alongside the `password_reset_grant_token`
// from Step 2. The server verifies the token, hashes the new password, updates
// the database, and returns a session token (auto-login).
//
// # Security: User ID Enumeration & Timing attack mitigation
// The grant token payload contains the `UserID`. If the UserID is not found in the
// database, we silently initialize a dummy user hash (`crypto.DummyPasswordHash`).
// We then proceed unconditionally to the HMAC verification. This guarantees the
// request always takes the same CPU time, and we uniformly return
// `errorJwtInvalidVerificationToken` regardless of whether the UserID was fake
// or the signature was just bad.
func (a *App) ConfirmPasswordResetOtpHandler(w http.ResponseWriter, r *http.Request) {
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

	if req.Token == "" || req.Password == "" || req.PasswordConfirm == "" {
		WriteJsonError(w, errorMissingFields)
		return
	}

	if req.Password != req.PasswordConfirm {
		WriteJsonError(w, errorPasswordMismatch)
		return
	}

	if err := a.Validator().Password(req.Password); err != nil {
		WriteJsonError(w, errorWeakPassword)
		return
	}

	// 1. Parse Token Unverified
	claims, err := crypto.ParseJwtUnverified(req.Token)
	if err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	if err := crypto.ValidatePasswordResetClaims(claims); err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	userID, _ := claims[crypto.ClaimUserID].(string)
	email, _ := claims[crypto.ClaimEmail].(string)

	// 2. Database Lookup & Timing Equalization
	user, err := a.DbAuth().GetUserById(userID)

	passwordHash := crypto.DummyPasswordHash
	if err == nil && user != nil {
		passwordHash = user.Password
	}

	// 3. Verify Grant Token Signature (Constant-Time Gate)
	cfg := a.Config()
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(
		email,
		passwordHash,
		cfg.Jwt.PasswordResetSecret,
	)
	if err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	_, err = crypto.ParseJwt(req.Token, signingKey)
	if err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	if user == nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// 4. Update Database
	hashedPassword, err := crypto.GenerateHash(req.Password)
	if err != nil {
		WriteJsonError(w, errorPasswordHashingFailed)
		return
	}

	if crypto.CheckPassword(req.Password, user.Password) {
		WriteJsonOk(w, okPasswordResetNotNeeded)
		return
	}

	err = a.DbAuth().UpdatePassword(user.ID, string(hashedPassword))
	if err != nil {
		WriteJsonError(w, errorServiceUnavailable)
		return
	}

	// 5. Response (No Auto-login)
	WriteJsonOk(w, okPasswordReset)
}
