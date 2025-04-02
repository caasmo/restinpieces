package core

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/queue"
)

// RequestEmailChangeHandler handles email change requests
// Endpoint: POST /api/request-email-change
// Authenticated: Yes (requires valid auth token)
// Allowed Mimetype: application/json
func (a *App) RequestEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}

	// Authenticate the user using the token from the request
	user, err, authResp := a.Authenticate(r)
	if err != nil {
		writeJsonError(w, authResp)
		return
	}

	if !user.Verified {
		writeJsonError(w, errorUnverifiedEmail)
		return
	}

	var req struct {
		NewEmail string `json:"new_email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Validate new email
	if req.NewEmail == "" {
		writeJsonError(w, errorMissingFields)
		return
	}

	// Check if new email is same as current
	if req.NewEmail == user.Email {
		writeJsonError(w, errorEmailConflict)
		return
	}

	// Validate email format
	if err := ValidateEmail(req.NewEmail); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Create queue payload
	// this is for uniqueness
	// use one request per bucket. 
	payload := queue.PayloadEmailChange{
		Email:          user.Email,
		CooldownBucket: queue.CoolDownBucket(a.config.RateLimits.EmailChangeCooldown, time.Now()),
	}

	payloadExtra := queue.PayloadEmailChangeExtra{
		NewEmail: req.NewEmail,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	payloadExtraBytes, err := json.Marshal(payloadExtra)
	if err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Insert job into queue
	err = a.db.InsertJob(queue.Job{
		JobType:      queue.JobTypeEmailChange,
		Payload:      payloadBytes,
		PayloadExtra: payloadExtraBytes,
		Status:       queue.StatusPending,
		Attempts:     0,
		MaxAttempts:  3,
	})
	if err != nil {
		writeJsonError(w, errorAuthDatabaseError)
		return
	}

	writeJsonOk(w, okEmailChange)
}

func (a *App) ConfirmEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}

	type request struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Validate required fields
	if req.Token == "" || req.Password == "" {
		writeJsonError(w, errorMissingFields)
		return
	}

	// Parse unverified claims to discard fast
	claims, err := crypto.ParseJwtUnverified(req.Token)
	if err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Validate all required claims exist and have correct values
	if err := crypto.ValidateEmailChangeClaims(claims); err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Get user from database to get password hash for signing key
	user, err := a.db.GetUserById(claims[crypto.ClaimUserID].(string))
	if err != nil || user == nil {
		writeJsonError(w, errorNotFound)
		return
	}

	// Verify password matches current password
	if !crypto.CheckPassword(req.Password, user.Password) {
		writeJsonError(w, errorInvalidCredentials)
		return
	}

	// Verify token signature using email change secret
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(
		claims[crypto.ClaimEmail].(string),
		user.Password,
		a.config.Jwt.EmailChangeSecret,
	)
	if err != nil {
		writeJsonError(w, errorTokenGeneration)
		return
	}

	// Fully verify token signature and claims
	_, err = crypto.ParseJwt(req.Token, signingKey)
	if err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Get new email from claims
	newEmail := claims["new_email"].(string)

	// Validate new email format (even though claims were validated, this is an extra check)
	if err := ValidateEmail(newEmail); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Update email in database
	err = a.db.UpdateEmail(user.ID, newEmail)
	if err != nil {
		writeJsonError(w, errorServiceUnavailable)
		return
	}

	writeJsonOk(w, okEmailChangeRequested)
}
