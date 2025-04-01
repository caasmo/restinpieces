package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
)

// RequestPasswordResetHandler handles password reset requests
// Endpoint: POST /request-password-reset
// Authenticated: No
// Allowed Mimetype: application/json
func (a *App) RequestPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}

	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		writeJsonError(w, errorInvalidRequest)
		return
	}
	if err := ValidateEmail(req.Email); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Check if email exists in system
    // this is error of db, return internal server error
    // TODO
	user, err := a.db.GetUserByEmail(req.Email)
	if err != nil {
		writeJsonError(w, errorNotFound)
		return
	}

    // user not found
	if user == nil {
		// Return success even if email doesn't exist to prevent email enumeration
		writeJsonOk(w, okPasswordResetRequested)
		return
	}

	// Calculate cooldown bucket for rate limiting
	cooldownBucket := queue.CoolDownBucket(a.config.RateLimits.PasswordResetCooldown, time.Now())

    // Create queue job with cooldown bucket. Second insertion in same bucket
    // will fail because unique
	payload, _ := json.Marshal(queue.PayloadPasswordReset{
		Email:          req.Email,
		CooldownBucket: cooldownBucket,
	})
	job := queue.Job{
		JobType: queue.JobTypePasswordReset,
		Payload: payload,
	}

	// Insert into job queue with deduplication
	err = a.db.InsertJob(job)
	if err != nil {
		if err == db.ErrConstraintUnique {
			writeJsonError(w, errorPasswordResetAlreadyRequested)
			return
		}
		writeJsonError(w, errorServiceUnavailable)
		return
	}

	writeJsonOk(w, okPasswordResetRequested)
}

// ConfirmPasswordResetHandler handles password reset confirmation
// Endpoint: POST /confirm-password-reset
// Authenticated: No
// Allowed Mimetype: application/json
func (a *App) ConfirmPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}

	type request struct {
		Token           string `json:"token"`
		Password        string `json:"password"`
		PasswordConfirm string `json:"password_confirm"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Validate required fields
	if req.Token == "" || req.Password == "" || req.PasswordConfirm == "" {
		writeJsonError(w, errorMissingFields)
		return
	}

	// Validate password match
	if req.Password != req.PasswordConfirm {
		writeJsonError(w, errorPasswordMismatch)
		return
	}

	// Validate password complexity
	if len(req.Password) < 8 {
		writeJsonError(w, errorPasswordComplexity)
		return
	}

	// Parse unverified claims to discard fast
	claims, err := crypto.ParseJwtUnverified(req.Token)
	if err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Validate all required claims exist and have correct values
	if err := crypto.ValidatePasswordResetClaims(claims); err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Get user from database to get password hash for signing key
	user, err := a.db.GetUserById(claims[crypto.ClaimUserID].(string))
	if err != nil || user == nil {
		writeJsonError(w, errorNotFound)
		return
	}

	// Verify token signature using password reset secret
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(
		claims[crypto.ClaimEmail].(string),
		user.Password,
		a.config.Jwt.PasswordResetSecret,
	)
	if err != nil {
		writeJsonError(w, errorPasswordResetFailed)
		return
	}

	// Fully verify token signature and claims
	_, err = crypto.ParseJwt(req.Token, signingKey)
	if err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Hash new password before storage
	hashedPassword, err := crypto.GenerateHash(req.Password)
	if err != nil {
		writeJsonError(w, errorTokenGeneration)
		return
	}

	// Update user password
	err = a.db.UpdatePassword(user.ID, string(hashedPassword))
	if err != nil {
		writeJsonError(w, errorServiceUnavailable)
		return
	}
	writeJsonOk(w, okPasswordReset)
}

