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

// RequestPasswordResetOtpHandler handles password reset OTP requests.
// Endpoint: POST /request-password-reset-otp
// Authenticated: No
// Allowed Mimetype: application/json
//
// # Flow
// Unauthenticated users can request a password reset OTP. The server queues an
// email containing a 6-digit OTP and immediately returns a stateless
// `verification_token` (JWT) to the client. This token encapsulates the hashed OTP.
//
// # Security: Harassment and Rate Limiting
// Because this endpoint sits completely unauthenticated and does not require the
// user's password, we cannot prevent harassment entirely. Our defense is strictly
// bounded by the queue Cooldown Bucket (e.g., 1 email every 5 minutes). Successive
// requests within the cooldown window hit a unique constraint and are silently ignored.
//
// # Security: Enumeration hardening
// If we return errors for invalid states (e.g., email not found, unverified), an
// attacker can enumerate valid accounts. Therefore, this handler employs a
// "silent state machine": it sets a boolean flag and never returns early.
//
// # Security: Timing attack mitigation
// If the email is fake, skipping the OTP cryptographic generation and the
// database INSERT would make the response extremely fast (~10ms) compared to a
// real user (~150ms).
// Mitigation 1: Unconditionally generate the OTP and its JWT.
// Mitigation 2: Unconditionally insert a job into the queue (`JobTypeDummy` for
// invalid states) to guarantee constant time disk/network execution.
func (a *App) RequestPasswordResetOtpHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
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

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}
	if err := a.Validator().Email(req.Email); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	cfg := a.Config()

	// 1. Unconditional Crypto Generation (Timing Equalizer)
	otp, verificationToken, err := crypto.NewJwtEmailOtpVerificationToken(
		req.Email,
		cfg.Jwt.PasswordResetSecret, // Reusing this secret for the verification token
		cfg.Jwt.PasswordResetTokenDuration.Duration,
	)
	if err != nil {
		WriteJsonError(w, errorOtpFailed)
		return
	}

	// 2. Database Lookup
	user, userErr := a.DbAuth().GetUserByEmail(req.Email)

	// 3. Silent State Machine (No Early Returns)
	shouldSendEmail := true
	if userErr != nil || user == nil {
		shouldSendEmail = false
	} else if !user.Verified {
		shouldSendEmail = false
	} else if user.Password == "" {
		shouldSendEmail = false
	}

	// 4. Unconditional Queue Insertion (Timing Equalizer)
	var jobType string
	var payload, payloadExtra []byte

	if shouldSendEmail {
		jobType = handlers.JobTypePasswordResetOtp
		cooldownBucket := queue.CoolDownBucket(cfg.RateLimits.PasswordResetCooldown.Duration, time.Now())
		payload, _ = json.Marshal(handlers.PayloadPasswordResetOtp{
			Email:          req.Email,
			CooldownBucket: cooldownBucket,
		})
		payloadExtra, _ = json.Marshal(handlers.PayloadPasswordResetOtpExtra{
			Otp: otp,
		})
	} else {
		jobType = handlers.JobTypeDummy
		// Ensure it never hits a unique constraint by using random data
		randomID := crypto.RandomString(32, crypto.AlphanumericAlphabet)
		payload, _ = json.Marshal(handlers.PayloadDummy{
			DummyID: randomID,
		})
	}

	job := db.Job{
		JobType:      jobType,
		Payload:      payload,
		PayloadExtra: payloadExtra,
	}

	// Log internal DB errors if needed, but DO NOT leak them to the client.
	_ = a.DbQueue().InsertJob(job)

	// 5. Uniform Response
	writeOtpResponse(w, verificationToken)
}
