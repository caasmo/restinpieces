package core

import (
	"encoding/json"
	"net/http"
	"strings"

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
// The dominant cost in this handler is crypto.VerifyPassword (argon2id, ~100ms).
// If the user lookup fails and we return immediately — skipping VerifyPassword —
// the response time is orders of magnitude shorter than a failed password check.
// An attacker can exploit this difference to enumerate valid emails with high
// confidence without ever needing the correct password: fast response means the
// email does not exist, slow response means it does.
//
// Mitigation: crypto.VerifyPassword is always called, even when the user is not
// found. On the not-found path it runs against a static dummy hash and its
// result is discarded. This ensures both paths pay the same argon2id cost and
// are indistinguishable by response time. VerifyPassword itself also burns one
// comparison when the stored record is unusable, so a corrupt record does not
// short-circuit the cost; the handler logs that detail and answers with the
// same errorInvalidCredentials as a wrong password.
//
// # Security: Verified check ordering
//
// The verified check runs after VerifyPassword deliberately. Checking it before
// would re-introduce a timing leak: a fast rejection for unverified accounts
// (before argon2id) vs a slow rejection for wrong passwords (after argon2id)
// would again allow email enumeration for accounts that exist but are
// unverified. Paying the full argon2id cost before checking verified status
// closes that gap.
//
// errorRequiredEmailVerificationOtp is the intentional UX escape for users who
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

	// RFC 5321 technically makes the local part case-sensitive
	// (User@example.com ≠ user@example.com), but in reality virtually every
	// mail server treats it case-insensitively. OWASP and most auth guidance
	// say: normalize to lowercase before storing and before lookup
	req.Identity = strings.ToLower(strings.TrimSpace(req.Identity))

	// What NIST SP 800-63B §5.1.1.2 says about spaces
	// "Verifiers SHOULD permit claimants to use any printable ASCII characters as well as the space character in memorized secrets."
	// "Verifiers MAY remove leading and trailing whitespace prior to verification."
	// we consistently add the same to the register handler
	req.Password = strings.TrimSpace(req.Password)

	if req.Identity == "" || req.Password == "" {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	if err := a.Validator().Email(req.Identity); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// One global bucket for the whole app, not a per-account throttle, so its
	// rejection is the same for every account. Placed before the lookup, a
	// blocked request costs no DB hit and no argon2id work.
	if a.Limiter().Limit(LimiterPasswordHashing) {
		WriteJsonError(w, errorTooManyRequests)
		return
	}

	user, err := a.DbAuth().GetUserByEmail(req.Identity)

	passwordHash := crypto.DummyPasswordHash
	if err == nil && user != nil {
		passwordHash = user.Password
	}

	// Always runs — see timing attack doc above.
	passwordValid, verifyErr := crypto.VerifyPassword(req.Password, passwordHash)

	if err != nil || user == nil {
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	// An unusable record is a data bug, not a wrong password. VerifyPassword
	// equalized the cost; log the detail and answer like a wrong password.
	if verifyErr != nil {
		a.Logger().Error("stored password record is unusable", "error", verifyErr)
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	if !passwordValid {
		WriteJsonError(w, errorInvalidCredentials)
		return
	}

	if !user.Verified {
		WriteJsonError(w, errorRequiredEmailVerificationOtp)
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
