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

// RequestEmailVerificationOtpHandler handles email OTP verification code requests.
// Endpoint: POST /request-email-otp-verification
// Authenticated: No
// Allowed Mimetype: application/json
//
// # Flow
//
// Called by the SDK immediately after Register or Login Handler. The caller
// must supply the same password used during registration. This is the only
// gate that matters: a valid password proves the caller went to registration
// and is the account owner.
//
// # Stateless OTP via verification token
//
// The 6-digit OTP is HMAC'd into a signed JWT (the verification token).
// The server stores nothing — no DB column, no cache, no session. On success
// the token is returned to the SDK, which holds it in memory and sends it
// back alongside the user-entered OTP at the confirm step.
//
// The password cannot replace this role: it proves identity but does not
// contain the OTP. Without the signed token there is no stateless way to
// verify which 6-digit code was issued. The alternative would require a
// server-side otp_hash column, an expiration column, and a cleanup job —
// trading the stateless design for statefulness with no security gain.
//
// # Security: Persistent harassment
//
// Requiring the correct password closes the harassment vector entirely.
// An attacker who knows a target email but not the password receives an 
// error and no email is ever sent. The legitimate user is the only one who can
// trigger mail delivery — the password is proof of prior registration.
//
// Unlike the register endpoint, CreateUserWithPassword never overwrites an
// existing password on conflict, so the real user's secret is always intact
// and cannot be poisoned by an attacker registering the same email.
//
// # Security: Enumeration & timing attacks
//
// The dominant cost in this handler is crypto.CheckPassword (bcrypt, ~100ms).
// If the user lookup fails and we return immediately — skipping CheckPassword —
// the response time is orders of magnitude shorter than a failed password check.
// An attacker can exploit this difference to enumerate valid emails with high
// confidence: fast response means the email does not exist, slow response means
// it does.
//
// Mitigation: crypto.CheckPassword is always called, even when the user is not
// found. On the not-found path it runs against a static dummy hash and its
// result is discarded. This ensures both paths pay the same bcrypt cost and
// are indistinguishable by response time.
//
// # Security: errorWeakPassword response
//
// The password validator returns a distinct errorWeakPassword before the DB
// lookup. This does not leak email existence — it only reveals that the input
// violates the password policy. The policy itself is not secret (the register
// handler exposes identical validation), so no information is gained.
//
// # Security: okAlreadyVerified response
//
// The already-verified check returns a distinct okAlreadyVerified only after
// the password gate. An attacker who can reach this branch already has the
// correct password and could simply log in. No information is gained that
// the attacker does not already possess.
func (a *App) RequestEmailVerificationOtpHandler(w http.ResponseWriter, r *http.Request) {
	resp, err := a.Validator().ContentType(r, MimeTypeJSON)
	if err != nil {
		WriteJsonError(w, resp)
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Password = strings.TrimSpace(req.Password)

	if req.Email == "" || req.Password == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if err := a.Validator().Email(req.Email); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if err := a.Validator().Password(req.Password); err != nil {
		WriteJsonError(w, errorWeakPassword)
		return
	}

	user, userErr := a.DbAuth().GetUserByEmail(req.Email)

	passwordHash := crypto.DummyPasswordHash
	if userErr == nil && user != nil {
		passwordHash = user.Password
	}

	// Always runs — see timing attack doc above.
	passwordValid := crypto.CheckPassword(req.Password, passwordHash)

	if userErr != nil || user == nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if !passwordValid {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if user.Verified {
		WriteJsonOk(w, okAlreadyVerified)
		return
	}

	cfg := a.Config()

	// Generate the OTP token.
	otp, verificationToken, err := crypto.NewJwtEmailOtpToken(
		req.Email,
		cfg.Jwt.VerificationEmailOtpSecret,
		cfg.Jwt.VerificationEmailOtpTokenDuration.Duration,
	)
	if err != nil {
		WriteJsonError(w, errorOtpFailed)
		return
	}

	cooldownBucket := queue.CoolDownBucket(cfg.RateLimits.EmailVerificationOtpCooldown.Duration, time.Now())

	payload, err := json.Marshal(handlers.PayloadEmailVerificationOtp{
		Email:          req.Email,
		CooldownBucket: cooldownBucket,
	})
	if err != nil {
		WriteJsonError(w, errorServiceUnavailable)
		return
	}

	payloadExtra, err := json.Marshal(handlers.PayloadEmailVerificationOtpExtra{
		Otp: otp,
	})
	if err != nil {
		WriteJsonError(w, errorServiceUnavailable)
		return
	}

	job := db.Job{
		JobType:      handlers.JobTypeEmailVerificationOtp,
		Payload:      payload,
		PayloadExtra: payloadExtra,
	}

	if err := a.DbQueue().InsertJob(job); err != nil {
		if err == db.ErrConstraintUnique {
			WriteJsonError(w, errorEmailVerificationOtpAlreadyRequested)
			return
		}
		WriteJsonError(w, errorAuthDatabaseError)
		return
	}

	writeOtpResponse(w, verificationToken)
}

