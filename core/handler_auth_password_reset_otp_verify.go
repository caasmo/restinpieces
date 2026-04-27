package core

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
)

// VerifyPasswordResetOtpHandler handles the second step of password reset.
// Endpoint: POST /verify-password-reset-otp
// Authenticated: No
// Allowed Mimetype: application/json
//
// # Flow
// The user submits the 6-digit OTP along with the `verification_token` from Step 1.
// If valid, the server returns a temporary `password_reset_grant_token` which
// authorizes the final password change.
//
// # Security: Enumeration hardening
// This handler uniformly returns `errorInvalidOtp` if anything fails:
// bad token signature, expired token, incorrect OTP, or user not found.
//
// # Security: The Password Hash Binding
// The generated `password_reset_grant_token` is signed using a composite key
// derived from the global `PasswordResetSecret` AND the user's *current password hash*.
// This guarantees that the exact millisecond the user successfully changes their
// password in Step 3, the hash changes, and the grant token instantly becomes
// cryptographically invalid, preventing any replay attacks.
func (a *App) VerifyPasswordResetOtpHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	var req struct {
		Otp               string `json:"otp"`
		VerificationToken string `json:"verification_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if req.Otp == "" || req.VerificationToken == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	cfg := a.Config()

	// 1. Cryptographic Gate
	email, err := crypto.VerifyEmailOtpToken(req.Otp, req.VerificationToken, cfg.Jwt.PasswordResetSecret)
	if err != nil {
		WriteJsonError(w, errorInvalidOtp)
		return
	}

	// 2. Database Lookup & State Verification
	user, err := a.DbAuth().GetUserByEmail(email)
	if err != nil || user == nil {
		WriteJsonError(w, errorInvalidOtp)
		return
	}
	if !user.Verified || user.Password == "" {
		WriteJsonError(w, errorInvalidOtp)
		return
	}

	// 3. Generate Grant Token (Password Hash Bound)
	grantToken, err := crypto.NewJwtPasswordResetToken(
		user.ID,
		user.Email,
		user.Password,
		cfg.Jwt.PasswordResetSecret,
		cfg.Jwt.PasswordResetTokenDuration.Duration,
	)
	if err != nil {
		WriteJsonError(w, errorServiceUnavailable)
		return
	}

	// 4. Response
	writePasswordResetOtpVerifiedResponse(w, grantToken)
}
