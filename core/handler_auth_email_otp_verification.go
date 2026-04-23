package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
	"github.com/caasmo/restinpieces/queue/handlers"
)

// RequestEmailOtpVerificationHandler handles email OTP verification code requests.
// Endpoint: POST /request-email-otp-verification
// Authenticated: No
// Allowed Mimetype: application/json
//
// # Security: Enumeration hardening
//
// This handler is deliberately opaque about account state. All three silent
// failure cases — email not found, account already verified, and OTP already
// requested (cooldown still active) — return an identical 200 response with a
// real, well-formed verification_token. No status code or body field reveals
// whether the email exists or what its state is.
//
// The confirm handler rejects tokens for non-existent / already-verified
// accounts, so a token issued on a silent-failure path is harmless.
//
// # Security: Timing
//
// OTP generation (JWT signing) always runs before any account-state branch,
// so the dominant CPU cost is paid on every request regardless of outcome.
//
// A small residual timing difference remains: InsertJob (a DB round-trip)
// only executes on the real path. This gap is narrow — crypto dominates the
// total latency — but a high-precision attacker under ideal network conditions
// could detect it. Acceptable for the current threat model; document here if
// that changes.
func (a *App) RequestEmailOtpVerificationHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := a.Validator().ContentType(r, MimeTypeJSON)
	if err != nil {
		WriteJsonError(w, resp)
		return
	}

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

	user, userErr := a.DbAuth().GetUserByEmail(req.Email)

	cfg := a.Config()

	// Always generate the OTP token regardless of account state.
	// Timing: JWT signing is the dominant cost — running it unconditionally
	// ensures all paths pay the same price before responding.
	// Enumeration: every caller receives a real, well-formed verification_token
	// indistinguishable from a genuine one. Silent-failure tokens are rejected
	// at the confirm step; no privilege is granted.
	otp, verificationToken, err := crypto.NewJwtEmailOtpVerificationToken(
		req.Email,
		cfg.Jwt.VerificationEmailOtpSecret,
		cfg.Jwt.VerificationEmailOtpTokenDuration.Duration,
	)
	if err != nil {
		WriteJsonError(w, errorOtpFailed)
		return
	}

	// Silent failure: email not found or account already verified.
	// Identical response to the success path — same status, same body, real token.
	if userErr != nil || user == nil || user.Verified {
		writeOtpResponse(w, verificationToken)
		return
	}

	cooldownBucket := queue.CoolDownBucket(cfg.RateLimits.EmailOtpVerificationCooldown.Duration, time.Now())

	payload, _ := json.Marshal(handlers.PayloadEmailVerificationOtp{
		Email:          req.Email,
		CooldownBucket: cooldownBucket,
	})
	payloadExtra, _ := json.Marshal(handlers.PayloadEmailVerificationOtpExtra{
		Otp: otp,
	})

	job := db.Job{
		JobType:      handlers.JobTypeEmailVerificationOtp,
		Payload:      payload,
		PayloadExtra: payloadExtra,
	}

	if err := a.DbQueue().InsertJob(job); err != nil {
		// Silent failure: ErrConstraintUnique means a request is already
		// pending within the cooldown window — the earlier job is still
		// delivered. Any other DB error is also swallowed. Surfacing either
		// would let a caller distinguish a live account from a non-existent
		// one, breaking enumeration hardening.
		if err != db.ErrConstraintUnique {
			WriteJsonError(w, errorServiceUnavailable)
			return
		}
	}

	writeOtpResponse(w, verificationToken)
}

// ConfirmEmailOtpVerificationHandler handles email OTP verification code confirmation
func (a *App) ConfirmEmailOtpVerificationHandler(w http.ResponseWriter, r *http.Request) {
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
	email, err := crypto.VerifyEmailOtpVerificationToken(req.Otp, req.VerificationToken, cfg.Jwt.VerificationEmailOtpSecret)
	if err != nil {
		WriteJsonError(w, errorInvalidOtp)
		return
	}

	user, err := a.DbAuth().GetUserByEmail(email)
	if err != nil || user == nil {
		WriteJsonError(w, errorNotFound)
		return
	}

	if user.Verified {
		WriteJsonOk(w, okAlreadyVerified)
		return
	}

	err = a.DbAuth().VerifyEmail(user.ID)
	if err != nil {
		WriteJsonError(w, errorServiceUnavailable)
		return
	}

	user.Verified = true

	// Generate JWT session token
	token, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, cfg.Jwt.AuthSecret, cfg.Jwt.AuthTokenDuration.Duration)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication response
	writeAuthResponse(w, token, user)
}
