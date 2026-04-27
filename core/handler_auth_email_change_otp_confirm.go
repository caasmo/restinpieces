package core

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue/handlers"
)

// ConfirmEmailChangeOtpHandler handles the final step of email change.
// Endpoint: POST /api/confirm-email-change-otp
// Authenticated: Yes (requires valid auth token)
// Allowed Mimetype: application/json
//
// # Flow
// The user submits the 6-digit OTP alongside the `verification_token` from
// Step 1. The server verifies the JWT signature and the OTP hash, then
// updates the email in the database. After a successful update, a security
// alert is queued to the old email address.
//
// # Security: Session invalidation as replay protection
// After UpdateEmail, the session signing key changes (derived from email +
// passwordHash + secret). The old session JWT becomes cryptographically
// invalid. This prevents replay of the confirm request and forces re-login.
//
// # Security: Race condition defense
// If the new email was taken between request and confirm, UpdateEmail fails
// with a unique constraint violation. This is returned as a generic error.
//
// # Security: Old email notification
// After a successful email update, a JobTypeEmailChangeAlert job is queued
// to inform the old email owner. This is best-effort — if the queue insert
// fails, the email change still succeeds (the DB update already committed).
func (a *App) ConfirmEmailChangeOtpHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := a.Validator().ContentType(r, MimeTypeJSON)
	if err != nil {
		WriteJsonError(w, resp)
		return
	}

	user, authResp, err := a.Auth().Authenticate(r)
	if err != nil {
		WriteJsonError(w, authResp)
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

	newEmail, err := crypto.VerifyEmailOtpToken(req.Otp, req.VerificationToken, cfg.Jwt.EmailChangeOtpSecret)
	if err != nil {
		WriteJsonError(w, errorInvalidOtp)
		return
	}

	if err := a.Validator().Email(newEmail); err != nil {
		WriteJsonError(w, errorInvalidOtp)
		return
	}

	if newEmail == user.Email {
		WriteJsonError(w, errorInvalidOtp)
		return
	}

	oldEmail := user.Email

	err = a.DbAuth().UpdateEmail(user.ID, newEmail)
	if err != nil {
		WriteJsonError(w, errorServiceUnavailable)
		return
	}

	// Best-effort: queue security alert to old email.
	// Failure does not affect the response — the email change already committed.
	alertPayload, _ := json.Marshal(handlers.PayloadEmailChangeAlert{
		OldEmail: oldEmail,
		NewEmail: newEmail,
	})
	_ = a.DbQueue().InsertJob(db.Job{
		JobType: handlers.JobTypeEmailChangeAlert,
		Payload: alertPayload,
	})

	WriteJsonOk(w, okEmailChange)
}
