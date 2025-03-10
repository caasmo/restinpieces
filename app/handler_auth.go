package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"
	
	"github.com/caasmo/restinpieces/crypto"
	"github.com/caasmo/restinpieces/db"
	"github.com/caasmo/restinpieces/queue"
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
	errorNotFound            = jsonError{http.StatusNotFound, []byte(`{"error":"Email not found"}`)}
	errorConflict            = jsonError{http.StatusConflict, []byte(`{"error":"Verification already requested"}`)}
	errorRegistrationFailed  = jsonError{http.StatusBadRequest, []byte(`{"error":"Registration failed"}`)}
	errorTooManyRequests     = jsonError{http.StatusTooManyRequests, []byte(`{"error":"Too many requests"}`)}
	errorServiceUnavailable  = jsonError{http.StatusServiceUnavailable, []byte(`{"error":"Service unavailable"}`)}
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
	if !isValidEmail(req.Identity) {
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


// isValidEmail performs RFC 5322 validation using net/mail
func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// RequestVerificationHandler handles email verification requests
func (a *App) RequestVerificationHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, errorInvalidRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || !isValidEmail(req.Email) {
		writeJSONError(w, errorInvalidRequest)
		return
	}

	// Check if email exists in system
	user, err := a.db.GetUserByEmail(req.Email)
	if err != nil || user == nil {
		writeJSONError(w, errorNotFound)
		return
	}

	// Create queue job
	payload, _ := json.Marshal(queue.EmailVerificationPayload(req.Email))
	job := queue.QueueJob{
		JobType:      queue.JobTypeEmailVerification,
		Payload:      payload,
		Status:       queue.StatusPending,
		MaxAttempts:  3,
		ScheduledFor: time.Now().UTC(),
	}

	// Insert into job queue with deduplication
	err = a.db.InsertQueueJob(job)
	if err != nil {
		if errors.Is(err, db.ErrConstraintUnique) {
			writeJSONError(w, errorConflict)
			return
		}
		writeJSONError(w, errorServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprint(w, `{"message":"email will be sent soon. Check your mailbox"}`)
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

// Grok 3
//

//CREATE TABLE job_queue (
//    id INTEGER PRIMARY KEY AUTOINCREMENT,
//    job_type TEXT NOT NULL,
//    payload TEXT NOT NULL,
//    status TEXT NOT NULL DEFAULT 'pending',
//    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
//    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
//);
//CREATE INDEX idx_status ON job_queue(status);
//CREATE INDEX idx_job_type ON job_queue(job_type);

// id: A unique identifier for each job, auto-incremented by SQLite.
// 
// job_type: A string indicating the type of job (e.g., "send_verification_email" for this task, or "process_payment" for another). This allows the table to support multiple job types.
// 
// payload: A text field storing job-specific data in a flexible format, such as JSON. For this task, it might be {"email": "user@example.com"}. Using TEXT keeps it general-purpose; JSON parsing can be handled in Go with the encoding/json package.
// 
// status: Tracks the job’s state, with values like:
// "pending": Job is queued and awaiting processing.
// 
// "processing": A worker has claimed the job.
// 
// "completed": Job finished successfully.
// 
// "failed": Job failed after processing.
// 
// Default is "pending".
// 
// created_at: Timestamp of when the job was added, useful for auditing and ordering.
// 
// updated_at: Timestamp of the last status update, helping track progress or detect stale jobs.


// To prevent multiple workers from processing the same job:
// Use an atomic update like:
// sql
// 
// UPDATE job_queue 
// SET status = 'processing', updated_at = CURRENT_TIMESTAMP 
// WHERE id = ? AND status = 'pending';

// claude
// 429 Too Many Requests: When rate limiting is triggered (e.g., too many requests from same IP/email)
// 409 Conflict: When a verification request for this email already exists and is pending

// Idempotency:
// Generate and accept idempotency keys
// Prevent duplicate requests within a time window

//Implement CSRF protection
//Set appropriate request size limits

// -- Main jobs queue table
// CREATE TABLE job_queue (
//     id INTEGER PRIMARY KEY AUTOINCREMENT,
//     job_type TEXT NOT NULL,  -- Type of job (email_verification, password_reset, etc.)
//     priority INTEGER DEFAULT 1, -- Higher number = higher priority
//     payload TEXT NOT NULL,   -- JSON payload with job-specific data
//     status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
//     attempts INTEGER NOT NULL DEFAULT 0, -- Number of processing attempts
//     max_attempts INTEGER NOT NULL DEFAULT 3, -- Maximum retry attempts
//     created_at TEXT NOT NULL DEFAULT (datetime('now')), -- ISO8601 string format
//     updated_at TEXT NOT NULL DEFAULT (datetime('now')), -- ISO8601 string format
//     scheduled_for TEXT NOT NULL DEFAULT (datetime('now')), -- When to process this job
//     locked_by TEXT,          -- Worker ID that claimed this job
//     locked_at TEXT,          -- When the job was claimed
//     completed_at TEXT,       -- When the job was completed
//     last_error TEXT,         -- Last error message if failed
//     
//     -- Indexes for efficient querying (using CREATE INDEX instead of inline INDEX)
// );
// 
// -- Create separate index statements
// CREATE INDEX idx_job_status ON job_queue (status, scheduled_for);
// CREATE INDEX idx_job_type ON job_queue (job_type, status);
// CREATE INDEX idx_locked_by ON job_queue (locked_by);
// 
// -- Job-specific metadata table
// CREATE TABLE job_metadata (
//     job_id INTEGER NOT NULL,
//     key TEXT NOT NULL,
//     value TEXT NOT NULL,
//     FOREIGN KEY (job_id) REFERENCES job_queue (id) ON DELETE CASCADE,
//     PRIMARY KEY (job_id, key)
// );
// 
// -- Rate limiting table to prevent abuse
// CREATE TABLE rate_limits (
//     identifier TEXT NOT NULL PRIMARY KEY, -- Can be email, IP, or combination
//     counter INTEGER NOT NULL DEFAULT 1,
//     reset_at TEXT NOT NULL,  -- ISO8601 datetime string
//     created_at TEXT NOT NULL DEFAULT (datetime('now')),
//     updated_at TEXT NOT NULL DEFAULT (datetime('now'))
// );
// 
// -- Job results table for maintaining history
// CREATE TABLE job_results (
//     id INTEGER PRIMARY KEY AUTOINCREMENT,
//     job_id INTEGER NOT NULL,
//     result_type TEXT NOT NULL, -- success, failure, etc.
//     result_data TEXT,          -- Result details in JSON
//     created_at TEXT NOT NULL DEFAULT (datetime('now')),
//     FOREIGN KEY (job_id) REFERENCES job_queue (id) ON DELETE CASCADE
// );

// email verification
// https://github.com/AfterShip/email-verifier
// but use first standard library net/mail 












