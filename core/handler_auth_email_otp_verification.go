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

// RequestEmailOtpVerificationHandler handles email OTP verification code requests
// Endpoint: POST /request-email-otp-verification
// Authenticated: No
// Allowed Mimetype: application/json
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

	user, err := a.DbAuth().GetUserByEmail(req.Email)
	if err != nil || user == nil {
		WriteJsonError(w, errorNotFound)
		return
	}

	if user.Verified {
		WriteJsonOk(w, okAlreadyVerified)
		return
	}

	cfg := a.Config()
	otp, verificationToken, err := crypto.NewJwtEmailOtpVerificationToken(
		req.Email,
		cfg.Jwt.VerificationEmailOtpSecret,
		cfg.Jwt.VerificationEmailOtpTokenDuration.Duration,
	)
	if err != nil {
		WriteJsonError(w, errorOtpFailed)
		return
	}

	// Calculate cooldown bucket for rate limiting
	cooldownBucket := queue.CoolDownBucket(cfg.RateLimits.EmailOtpVerificationCooldown.Duration, time.Now())

	// Enqueue OTP email job asynchronously
	// OTP goes into PayloadExtra to not break the unique index on (job_type, payload)
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
		if err == db.ErrConstraintUnique {
			WriteJsonError(w, errorEmailVerificationAlreadyRequested)
			return
		}
		WriteJsonError(w, errorServiceUnavailable)
		return
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

	WriteJsonOk(w, okOtpVerified)
}
