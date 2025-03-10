package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
)

//	export JWT_SECRET=$(openssl rand -base64 32)
//
//	First get a valid JWT token (replace JWT_SECRET with your actual secret)
//	This is a test token generation command using jwt-cli (install via 'go install github.com/matiaskorhonen/jwt-cli@latest')
//	JWT_TOKEN=$(jwt encode --secret "${JWT_SECRET}" --claim user_id=testuser123 --exp +5m)
//
//	# Test valid token refresh
//	curl -v -X POST http://localhost:8080/auth-refresh \
//	  -H "Authorization: Bearer $JWT_TOKEN"
//
//	# Test invalid token
//	curl -v -X POST http://localhost:8080/auth-refresh \
//	  -H "Authorization: Bearer invalid.token.here"
//
//	# Test missing header
//	curl -v -X POST http://localhost:8080/auth-refresh
//
// Precomputed error responses with status codes
var (
	errorTokenGeneration     = jsonError{http.StatusInternalServerError, []byte(`{"error":"Failed to generate token"}`)}
	errorClaimsNotFound      = jsonError{http.StatusInternalServerError, []byte(`{"error":"Failed to generate token: Claims not found"}`)}
	errorInvalidRequest      = jsonError{http.StatusBadRequest, []byte(`{"error":"Invalid request payload"}`)}
	errorInvalidCredentials  = jsonError{http.StatusUnauthorized, []byte(`{"error":"Invalid credentials"}`)}
	errorPasswordMismatch    = jsonError{http.StatusBadRequest, []byte(`{"error":"Password and confirmation do not match"}`)}
	errorMissingFields       = jsonError{http.StatusBadRequest, []byte(`{"error":"Missing required fields"}`)}
	errorPasswordComplexity  = jsonError{http.StatusBadRequest, []byte(`{"error":"Password must be at least 8 characters"}`)}
	errorEmailConflict       = jsonError{http.StatusConflict, []byte(`{"error":"Email already registered"}`)}
	errorRegistrationFailed  = jsonError{http.StatusBadRequest, []byte(`{"error":"Registration failed"}`)}
)

// RefreshAuthHandler handles explicit JWT token refresh requests
func (a *App) RefreshAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Get claims from context (added by JwtValidate middleware)
	userId, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userId == "" {
		writeJSONError(w, errorClaimsNotFound)
		return
	}

	// Generate new token with fresh expiration
	newToken, expiry, err := crypto.CreateJwt(userId, a.config.JwtSecret, a.config.TokenDuration)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	// Calculate seconds until expiry
	expiresIn := int(time.Until(expiry).Seconds())

	// Return new token in response following OAuth2 token exchange format
	w.Header()["Content-Type"] = jsonHeader

	// Standard OAuth2 token response format
	fmt.Fprintf(w, `{
		"token_type": "Bearer",
		"expires_in": %d,
		"access_token": "%s"
	}`, expiresIn, newToken)

}

// AuthWithPasswordHandler handles password-based authentication
func (a *App) AuthWithPasswordHandler(w http.ResponseWriter, r *http.Request) {

	var req struct {
		Identity string `json:"identity"` // username or email, only mail implemented
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, errorInvalidRequest)
		return
	}

	if req.Identity == "" || req.Password == "" {
		writeJSONError(w, errorInvalidRequest)
		return
	}

    // only email TODO
	if !isValidIdentity(req.Identity) {
		writeJSONError(w, errorInvalidRequest)
		return
	}

	// Get user from database
	user, err := a.db.GetUserByEmail(req.Identity)
	if err != nil || user == nil {
		writeJSONError(w, errorInvalidCredentials)
		return
	}

	// Verify password hash
	if !crypto.CheckPassword(req.Password, user.Password) {
		writeJSONError(w, errorInvalidCredentials)
		return
	}

	// Generate JWT token
	token, _, err := crypto.CreateJwt(user.ID, a.config.JwtSecret, a.config.TokenDuration)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"record": map[string]interface{}{
			"id":       user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"verified": user.Verified,
		},
	})
}


// isValidIdentity performs basic email format validation
// TODO better validation ozzo?
func isValidIdentity(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// RegisterHandler handles user registration with validation
func (a *App) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identity        string `json:"identity"`
		Password        string `json:"password"`
		PasswordConfirm string `json:"password_confirm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, errorInvalidRequest)
		return
	}

	// Validate required fields
	req.Identity = strings.TrimSpace(req.Identity)
	req.Password = strings.TrimSpace(req.Password)
	if req.Identity == "" || req.Password == "" || req.PasswordConfirm == "" {
		writeJSONError(w, errorMissingFields)
		return
	}

	// Validate password match
	if req.Password != req.PasswordConfirm {
		writeJSONError(w, errorPasswordMismatch)
		return
	}

	// Validate password complexity
	if len(req.Password) < 8 {
		writeJSONError(w, errorPasswordComplexity)
		return
	}

	// Hash password before storage
	hashedPassword, err := crypto.GenerateHash(req.Password)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	// Create user in database
	user, err := a.db.CreateUser(db.User{
		Email:     req.Identity,
		Password:  string(hashedPassword),
		Name:      "", // Optional field
		TokenKey:  crypto.GenerateSecureToken(32), // Generate secure token TODO 
	})

	if err != nil {
		// Handle unique constraint violation (email already exists)
		if err == db.ErrConstraintUnique {
			writeJSONError(w, errorEmailConflict)
			return
		}
		writeJSONErrorf(w, http.StatusInternalServerError, `{"error":"Registration failed: %s"}`, err.Error())
		return
	}

	// Generate JWT token for immediate authentication
	token, _, err := crypto.CreateJwt(user.ID, a.config.JwtSecret, a.config.TokenDuration)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"record": map[string]interface{}{
			"id":       user.ID,
			"email":    user.Email,
			"name":     user.Name,
			"verified": user.Verified,
		},
	})
}

// /request-verification endpoint

// r1
// 
// HTTP Status Codes:
// 
//  • 202 Accepted (Primary success response - indicates request accepted for processing)
//  • 400 Bad Request (Invalid/missing email format)
//  • 404 Not Found (Email not found in system - if you want to reveal existence)
//  • 429 Too Many Requests (Rate limiting)
//  • 500 Internal Server Error (Unexpected backend failures)
//  • 503 Service Unavailable (If email queue is overloaded)
// 
// Key Considerations:
// 
//  1 Validation Layer:
//     • Strict email format validation (RFC 5322 + DNS MX record check)
//     • Existence check in DB before queueing
//     • Rate limiting per IP/email (prevent abuse)
//  2 Security:
//     • Generic success response regardless of email existence ("If found, verification email sent")
//     • Input sanitization against SQLi
//     • Request timeout handling
//     • HMAC signature for queue jobs
//  3 Data Integrity:
//     • DB transactions (user check + queue insert atomic operation)
//     • Deduplication mechanism (unique constraint on email+timestamp)
//  4 Async Processing:
//     • Exponential backoff for failed email attempts
//     • Dead letter queue for permanent failures
//     • Idempotency keys in queue

// CREATE TABLE verification_queue (
//      id UUID PRIMARY KEY,
//      email VARCHAR(320) NOT NULL,
//      token_hash CHAR(64) NOT NULL, -- HMAC-SHA256 of verification token
//      scheduled_at TIMESTAMPTZ NOT NULL,
//      attempt_count INT DEFAULT 0,
//      last_attempt TIMESTAMPTZ,
//      status VARCHAR(20) DEFAULT 'pending'
//          CHECK (status IN ('pending', 'processing', 'sent', 'failed')),
//      created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
//      updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
//  );
// 
//  -- Indexes
//  CREATE INDEX idx_verification_pending ON verification_queue (scheduled_at)
//      WHERE status = 'pending';
//  CREATE UNIQUE INDEX idx_verification_dedupe ON verification_queue (email, token_hash);
// 
// 
// Additional Tables Needed:
//  -- For actual verification attempts
//  CREATE TABLE verification_tokens (
//      user_id UUID REFERENCES users(id),
//      token CHAR(64) PRIMARY KEY,
//      expires_at TIMESTAMPTZ NOT NULL,
//      consumed BOOLEAN DEFAULT false
//  );
// 
//  -- For rate limiting audit
//  CREATE TABLE verification_attempts (
//      email VARCHAR(320) NOT NULL,
//      attempt_ip INET NOT NULL,
//      attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
//  );











