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

// RequestVerificationHandler handles email verification requests
// Endpoint: POST /request-verification
// Authenticated: Yes
// Allowed Mimetype: application/json
func (a *App) RequestEmailVerificationHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}

	// Require authentication
	user, _, resp := a.Authenticate(r)
	if user == nil {
		writeJsonError(w, resp)
		return
	}

	// Check if user is already verified
	if user.Verified {
		writeJsonOk(w, okAlreadyVerified)
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

	// Verify the authenticated user matches the requested email
	if user.Email != req.Email {
		writeJsonError(w, errorEmailConflict)
		return
	}

	// Calculate cooldown bucket for rate limiting
	cfg := a.Config() // Get the current config
	cooldownBucket := queue.CoolDownBucket(cfg.RateLimits.EmailVerificationCooldown, time.Now())

	// Create queue job with cooldown bucket
	payload, _ := json.Marshal(queue.PayloadEmailVerification{
		Email:          req.Email,
		CooldownBucket: cooldownBucket,
	})
	job := queue.Job{
		JobType: queue.JobTypeEmailVerification,
		Payload: payload,
	}

	err := a.DbQueue().InsertJob(job)
	if err != nil {
		if err == db.ErrConstraintUnique {
			writeJsonError(w, errorEmailVerificationAlreadyRequested)
			return
		}
		writeJsonError(w, errorServiceUnavailable)
		return
	}

	writeJsonOk(w, okVerificationRequested)
}

func (a *App) ConfirmEmailVerificationHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		writeJsonError(w, resp)
		return
	}
	type request struct {
		Token string `json:"token"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJsonError(w, errorInvalidRequest)
		return
	}

	// Parse unverified claims to discrd fast
	claims, err := crypto.ParseJwtUnverified(req.Token)
	if err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Validate all required claims exist and have correct values
	if err := crypto.ValidateEmailVerificationClaims(claims); err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	user, err := a.DbAuth().GetUserById(claims[crypto.ClaimUserID].(string))
	if err != nil || user == nil {
		writeJsonError(w, errorNotFound)
		return
	}

	// Verify token signature using verification email secret
	cfg := a.Config() // Get the current config
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(
		claims[crypto.ClaimEmail].(string),
		user.Password,
		cfg.Jwt.VerificationEmailSecret,
	)
	if err != nil {
		writeJsonError(w, errorEmailVerificationFailed)
		return
	}

	// Fully verify token signature and claims
	_, err = crypto.ParseJwt(req.Token, signingKey)
	if err != nil {
		writeJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Check if user is already verified
	if user.Verified {
		writeJsonOk(w, okAlreadyVerified)
		return
	}

	err = a.DbAuth().VerifyEmail(user.ID)
	if err != nil {
		writeJsonError(w, errorServiceUnavailable)
		return
	}

	writeJsonOk(w, okEmailVerified)
}
