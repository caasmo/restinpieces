package core

import (
	"encoding/json"
	"net/http"

	"github.com/caasmo/restinpieces/crypto"
)

// AuthWithPasswordHandler handles password-based authentication (login).
// Endpoint: POST /auth-with-password
// Authenticated: No
// Allowed Mimetype: application/json
//
// # Security: Enumeration hardening
//
// The handler returns exactly two states to the caller:
//
//   - Success: credentials valid, account verified, session token issued.
//   - Failure: errorInvalidCredentials, for every credential failure without exception.
//
// Both "email not found" and "wrong password" collapse to errorInvalidCredentials.
// Distinguishing them would allow an attacker to confirm whether an email is
// registered by observing the error code alone.
//
// # Security: Timing attack
//
// The dominant cost in this handler is crypto.CheckPassword (bcrypt, ~100ms).
// If the user lookup fails and we return immediately — skipping CheckPassword —
// the response time is orders of magnitude shorter than a failed password check.
// An attacker can exploit this difference to enumerate valid emails with high
// confidence without ever needing the correct password: fast response means the
// email does not exist, slow response means it does.
//
// Mitigation: crypto.CheckPassword is always called, even when the user is not
// found. On the not-found path it runs against a static dummy hash and its
// result is discarded. This ensures both paths pay the same bcrypt cost and
// are indistinguishable by response time.
//
// # Security: Verified check ordering
//
// The verified check runs after CheckPassword deliberately. Checking it before
// would re-introduce a timing leak: a fast rejection for unverified accounts
// (before bcrypt) vs a slow rejection for wrong passwords (after bcrypt) would
// again allow email enumeration for accounts that exist but are unverified.
// Paying the full bcrypt cost before checking verified status closes that gap.
//
// errorRequiredEmailOtpVerification is the intentional UX escape for users who
// registered but did not complete email verification. It confirms the email
// exists and the password is correct — acceptable because this is functionally
// a login success gated on a verification step, not a credential failure.
func (a *App) AuthWithPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	var req struct {
		Identity string `json:"identity"` // email only, username reserved for future use
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if req.Identity == "" || req.Password == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if err := a.Validator().Email(req.Identity); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	user, err := a.DbAuth().GetUserByEmail(req.Identity)

	passwordHash := crypto.DummyPasswordHash
	if err == nil && user != nil {
		passwordHash = user.Password
	}

	// Always runs — see timing attack doc above.
	passwordValid := crypto.CheckPassword(req.Password, passwordHash)

	if err != nil || user == nil {
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	if !passwordValid {
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	if !user.Verified {
		WriteJsonError(w, errorRequiredEmailOtpVerification)
		return
	}

	cfg := a.Config()
	token, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, cfg.Jwt.AuthSecret, cfg.Jwt.AuthTokenDuration.Duration)
	if err != nil {
		WriteJsonError(w, errorTokenGeneration)
		return
	}

	writeAuthResponse(w, token, user)
}
