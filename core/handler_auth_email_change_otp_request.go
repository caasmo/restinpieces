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

// RequestEmailChangeOtpHandler handles email change OTP requests.
// Endpoint: POST /api/request-email-change-otp
// Authenticated: Yes (requires valid auth token + password re-auth)
// Allowed Mimetype: application/json
//
// # Flow
// Authenticated users request an email change by providing their current
// password and the new email address. The password is required as step-up
// authentication to protect against session hijack and physical access attacks.
// The server queues an email containing a 6-digit OTP to the new address and
// immediately returns a stateless `verification_token` (JWT) to the client.
// This token encapsulates the hashed OTP and the new email.
//
// # Security: Step-up authentication
// The current password is required to prevent account takeover via stolen
// sessions or physical access to an unlocked device. This follows the
// industry standard (Google, Apple, Microsoft, GitHub all require password
// re-entry for email changes).
//
// # Security: Enumeration hardening
// If the new email already belongs to another account, the handler enters the
// silent path: it still generates the OTP and JWT unconditionally, inserts a
// `JobTypeDummy` into the queue, and returns the same 200 response. The
// existence of the new email is never revealed.
//
// # Security: Timing attack mitigation
// OTP generation and queue insertion are unconditional to guarantee constant
// time regardless of the new email's state.
func (a *App) RequestEmailChangeOtpHandler(w http.ResponseWriter, r *http.Request) {
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

	if !user.Verified {
		WriteJsonError(w, errorUnverifiedEmail)
		return
	}

	var req struct {
		NewEmail string `json:"new_email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	req.NewEmail = strings.ToLower(strings.TrimSpace(req.NewEmail))
	if req.NewEmail == "" || req.Password == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if err := a.Validator().Email(req.NewEmail); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if !crypto.CheckPassword(req.Password, user.Password) {
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	if req.NewEmail == user.Email {
		WriteJsonError(w, errorEmailConflict)
		return
	}

	cfg := a.Config()

	// 1. Unconditional Crypto Generation (Timing Equalizer)
	otp, verificationToken, err := crypto.NewJwtEmailOtpToken(
		req.NewEmail,
		cfg.Jwt.EmailChangeOtpSecret,
		cfg.Jwt.EmailChangeOtpTokenDuration.Duration,
	)
	if err != nil {
		WriteJsonError(w, errorOtpFailed)
		return
	}

	// 2. Silent State Machine (No Early Returns After This Point)
	shouldSendEmail := true

	existingUser, existingErr := a.DbAuth().GetUserByEmail(req.NewEmail)
	if existingErr == nil && existingUser != nil {
		shouldSendEmail = false
	}

	// 3. Unconditional Queue Insertion (Timing Equalizer)
	var jobType string
	var payload, payloadExtra []byte

	if shouldSendEmail {
		jobType = handlers.JobTypeEmailChangeOtp
		cooldownBucket := queue.CoolDownBucket(cfg.RateLimits.EmailChangeCooldown.Duration, time.Now())
		payload, _ = json.Marshal(handlers.PayloadEmailChangeOtp{
			NewEmail:       req.NewEmail,
			CooldownBucket: cooldownBucket,
		})
		payloadExtra, _ = json.Marshal(handlers.PayloadEmailChangeOtpExtra{
			Otp: otp,
		})
	} else {
		jobType = handlers.JobTypeDummy
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

	_ = a.DbQueue().InsertJob(job)

	// 4. Uniform Response
	writeOtpResponse(w, verificationToken)
}
