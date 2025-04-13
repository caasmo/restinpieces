package core

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
)

// RequestEmailChangeHandler handles email change requests
// Endpoint: POST /api/request-email-change
// Authenticated: Yes (requires valid auth token)
// Allowed Mimetype: application/json
func (a *App) RequestEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	// Authenticate the user using the token from the request
	user, err, authResp := a.Authenticate(r)
	if err != nil {
		WriteJsonError(w, authResp)
		return
	}

	if !user.Verified {
		WriteJsonError(w, errorUnverifiedEmail)
		return
	}

	var req struct {
		NewEmail string `json:"new_email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Validate new email
	if req.NewEmail == "" {
		WriteJsonError(w, errorMissingFields)
		return
	}

	// Check if new email is same as current
	if req.NewEmail == user.Email {
		WriteJsonError(w, errorEmailConflict)
		return
	}

	// Validate email format
	if err := ValidateEmail(req.NewEmail); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Create queue payload
	// this is for uniqueness
	// use one request per bucket.
	cfg := a.Config() // Get the current config
	payload := queue.PayloadEmailChange{
		UserID:         user.ID,
		CooldownBucket: queue.CoolDownBucket(cfg.RateLimits.EmailChangeCooldown, time.Now()),
	}

	payloadExtra := queue.PayloadEmailChangeExtra{
		NewEmail: req.NewEmail,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	payloadExtraBytes, err := json.Marshal(payloadExtra)
	if err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	err = a.DbQueue().InsertJob(queue.Job{
		JobType:      queue.JobTypeEmailChange,
		Payload:      payloadBytes,
		PayloadExtra: payloadExtraBytes,
		Status:       queue.StatusPending,
		Attempts:     0,
		MaxAttempts:  3,
	})
	if err != nil {
		if err == db.ErrConstraintUnique {
			WriteJsonError(w, errorEmailChangeAlreadyRequested)
			return
		}
		WriteJsonError(w, errorAuthDatabaseError)
		return
	}

	WriteJsonOk(w, okEmailChangeRequested)
}

func (a *App) ConfirmEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
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
