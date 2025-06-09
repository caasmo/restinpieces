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
		WriteJsonError(w, resp)
		return
	}

    // validate request first
	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}
	if err := ValidateEmail(req.Email); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Require authentication
	user, _, resp := a.Auth().Authenticate(r)
	if user == nil {
		WriteJsonError(w, resp)
		return
	}

	// Check if user is already verified
	if user.Verified {
		WriteJsonOk(w, okAlreadyVerified)
		return
	}

	// Verify the authenticated user matches the requested email
	if user.Email != req.Email {
		WriteJsonError(w, errorEmailConflict)
		return
	}

	// Calculate cooldown bucket for rate limiting
	cfg := a.Config() // Get the current config
	cooldownBucket := queue.CoolDownBucket(cfg.RateLimits.EmailVerificationCooldown.Duration, time.Now())

	// Create queue job with cooldown bucket
	payload, _ := json.Marshal(queue.PayloadEmailVerification{
		Email:          req.Email,
		CooldownBucket: cooldownBucket,
	})
	job := db.Job{
		JobType: queue.JobTypeEmailVerification,
		Payload: payload,
	}

	err := a.DbQueue().InsertJob(job)
	if err != nil {
		if err == db.ErrConstraintUnique {
			WriteJsonError(w, errorEmailVerificationAlreadyRequested)
			return
		}
		WriteJsonError(w, errorServiceUnavailable)
		return
	}

	WriteJsonOk(w, okVerificationRequested)
}

func (a *App) ConfirmEmailVerificationHandler(w http.ResponseWriter, r *http.Request) {
	if err, resp := a.ValidateContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}
	type request struct {
		Token string `json:"token"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Parse unverified claims to discrd fast
	claims, err := crypto.ParseJwtUnverified(req.Token)
	if err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Validate all required claims exist and have correct values
	if err := crypto.ValidateEmailVerificationClaims(claims); err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	user, err := a.DbAuth().GetUserById(claims[crypto.ClaimUserID].(string))
	if err != nil || user == nil {
		WriteJsonError(w, errorNotFound)
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
		WriteJsonError(w, errorEmailVerificationFailed)
		return
	}

	// Fully verify token signature and claims
	_, err = crypto.ParseJwt(req.Token, signingKey)
	if err != nil {
		WriteJsonError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Check if user is already verified
	if user.Verified {
		WriteJsonOk(w, okAlreadyVerified)
		return
	}

	err = a.DbAuth().VerifyEmail(user.ID)
	if err != nil {
		WriteJsonError(w, errorServiceUnavailable)
		return
	}

	WriteJsonOk(w, okEmailVerified)
}
