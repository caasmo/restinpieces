package core

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
)

// ConfirmEmailVerificationOtpHandler handles email OTP verification code confirmation.
// Endpoint: POST /confirm-email-otp-verification
// Authenticated: No
// Allowed Mimetype: application/json
//
// # Stateless OTP verification
//
// The request handler HMAC'd the 6-digit OTP into a signed JWT (the
// verification token) and returned it to the SDK. This handler receives
// the user-entered OTP and the token, verifies the JWT signature, and
// checks that the OTP matches. The server never stored the OTP — the
// signed token is the only proof of what was issued.
//
// The password is not required here and cannot replace the verification
// token: the password proves identity, but it does not contain the OTP.
// Identity was already proven at the request step; what remains is
// proving possession of the email inbox (by entering the correct OTP).
//
// # Security: Enumeration hardening
//
// This handler returns exactly two states to the caller:
//
//   - Success: account is now verified, session token issued via writeAuthResponse.
//   - Failure: errorInvalidOtp, for every other case without exception.
//
// Failure is intentionally opaque. The following distinct internal conditions
// all map to errorInvalidOtp:
//
//   - OTP or verification token is cryptographically invalid or expired.
//   - Email extracted from the token does not match any account.
//   - Account is already verified.
//
// The only way to obtain a valid signed token is from our own request handler,
// which only issues tokens after verifying the correct password. The
// enumeration surface here is therefore narrower than at the request step.
func (a *App) ConfirmEmailVerificationOtpHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := a.Validator().ContentType(r, MimeTypeJSON)
	if err != nil {
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

	// Cryptographic gate. An invalid or expired token — including tokens that
	// were legitimately issued by the request handler for non-existent or
	// already-verified accounts — is rejected here uniformly.
	email, err := crypto.VerifyEmailOtpToken(req.Otp, req.VerificationToken, cfg.Jwt.VerificationEmailOtpSecret)
	if err != nil {
		WriteJsonError(w, errorInvalidOtp)
		return
	}

	user, err := a.DbAuth().UpdateVerified(email)
	if err != nil || user == nil || user.ID == "" {
		WriteJsonError(w, errorInvalidOtp)
		return
	}

	// Theoretical failure: VerifyEmail has already committed to the DB, so the
	// account is verified regardless of what happens here. If token generation
	// fails the user receives errorTokenGeneration but is not locked out — they
	// can obtain a session via the normal login flow. No corrective action is
	// needed; the inconsistency is transient.
	token, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, cfg.Jwt.AuthSecret, cfg.Jwt.AuthTokenDuration.Duration)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	writeAuthResponse(w, token, user)
}

