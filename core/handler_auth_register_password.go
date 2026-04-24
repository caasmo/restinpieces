package core

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
)

// RegisterWithPasswordHandler handles password-based user registration.
// Endpoint: POST /register-with-password
// Authenticated: No
// Allowed Mimetype: application/json
//
// # Security: Email Enumeration and Timing Attack Prevention
//
// This handler always returns the same response (okPendingEmailOtpVerification)
// regardless of whether the email already exists in the database, and regardless
// of whether the existing account used password or OAuth2 signup.
//
// This is intentional. Revealing different responses per case would allow an
// attacker to enumerate valid emails by observing response bodies.
//
// Timing attacks are also mitigated: crypto.GenerateHash (bcrypt/argon2) is
// always executed before the DB write, so the response time is dominated by
// the hash cost in all code paths. An attacker cannot infer email existence
// from response latency.
//
// # Password Protection on Conflict
//
// On email conflict, CreateUserWithPassword never updates the existing password.
// This prevents account takeover: an attacker who knows a valid email cannot
// overwrite the real user's password via this unauthenticated endpoint,
// regardless of whether the account was created with password or OAuth2.
// Changing a password requires authentication (dedicated settings endpoint).
//
// # Flow
//
// 1. Validate input.
// 2. Hash password (always, every code path).
// 3. Upsert user: insert on new email, no-op on conflict (password untouched).
// 4. Always return okPendingEmailOtpVerification.
//
// The SDK then calls RequestEmailOtpVerification. Email ownership proof via OTP
// is the gate. Whatever happened in the DB is irrelevant to the response here.
func (a *App) RegisterWithPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if resp, err := a.Validator().ContentType(r, MimeTypeJSON); err != nil {
		WriteJsonError(w, resp)
		return
	}

	var req struct {
		Identity string `json:"identity"`
		Password string `json:"password"`
		PasswordConfirm string `json:"password_confirm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	req.Identity = strings.TrimSpace(req.Identity)
	req.Password = strings.TrimSpace(req.Password)
	if req.Identity == "" || req.Password == "" || req.PasswordConfirm == "" {
		WriteJsonError(w, errorMissingFields)
		return
	}

	if err := a.Validator().Email(req.Identity); err != nil {
		WriteJsonError(w, errorInvalidRequest)
		return
	}

	// Validate password match
	if req.Password != req.PasswordConfirm {
		WriteJsonError(w, errorPasswordMismatch)
		return
	}

	if err := a.Validator().Password(req.Password); err != nil {
		WriteJsonError(w, errorWeakPassword)
		return
	}

	// Always hash, every code path — including when the email already exists.
	// Skipping the hash on conflict would make that path faster, leaking
	// email existence via response timing.
	hashedPassword, err := crypto.GenerateHash(req.Password)
	if err != nil {
		WriteJsonError(w, errorPasswordHashingFailed)
		return
	}

	newUser := db.User{
		Email:           req.Identity,
		Password:        string(hashedPassword),
		Verified:        false,
		Oauth2:          false,
		EmailVisibility: false,
	}

	// On email conflict, CreateUserWithPassword leaves the existing password
	// untouched (see SQL: ON CONFLICT DO UPDATE does not SET password).
	// We do not inspect the returned user — the response is always the same.
	if _, err := a.DbAuth().CreateUserWithPassword(newUser); err != nil {
		WriteJsonError(w, errorAuthDatabaseError)
		return
	}

	// Always returned: new user, existing password user, existing OAuth2 user.
	// The SDK proceeds to OTP verification in all cases. Email ownership is
	// the only gate that matters.
	WriteJsonOk(w, okPendingEmailOtpVerification)
}
