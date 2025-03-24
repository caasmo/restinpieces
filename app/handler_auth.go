package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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
//	Note: Use NewJwtSessionToken() instead of NewJwtSession
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

// RefreshAuthHandler handles explicit JWT token refresh requests
// Endpoint: POST /auth-refresh
func (a *App) RefreshAuthHandler(w http.ResponseWriter, r *http.Request) {
	// Get claims from context (added by JwtValidate middleware)
	userId, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userId == "" {
		slog.Error("Failed to get user ID from context")
		writeJSONError(w, errorClaimsNotFound)
		return
	}

	// Get user from database to get email for signing key
	user, err := a.db.GetUserById(userId)
	if err != nil || user == nil {
		slog.Error("Failed to fetch user", "user_id", userId, "error", err)
		writeJSONError(w, errorInvalidCredentials)
		return
	}

	// Generate new token with fresh expiration using NewJwtSession
	newToken, expiry, err := crypto.NewJwtSessionToken(userId, user.Email, user.Password, a.config.Jwt.AuthSecret, a.config.Jwt.AuthTokenDuration)
	if err != nil {
		slog.Error("Failed to generate new token", "error", err)
		writeJSONError(w, errorTokenGeneration)
		return
	}
	slog.Debug("New token generated",
		"expiry", expiry,
		"token_length", len(newToken))

	// Calculate seconds until expiry
	expiresIn := int(time.Until(expiry).Seconds())

	// Return new token in response following OAuth2 token exchange format
	w.Header()["Content-Type"] = jsonHeader

	// Standard OAuth2 token response format
	// TODO do we need the expires_in, remove from NewJwt
	fmt.Fprintf(w, `{
		"token_type": "Bearer",
		"expires_in": %d,
		"access_token": "%s"
	}`, expiresIn, newToken)

}

// AuthWithPasswordHandler handles password-based authentication (login)
// Endpoint: POST /auth-with-password
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

	// Validate email format
	if err := ValidateEmail(req.Identity); err != nil {
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

	// Generate JWT session token
	token, _, err := crypto.NewJwtSessionToken(user.ID, user.Email, user.Password, a.config.Jwt.AuthSecret, a.config.Jwt.AuthTokenDuration)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication token response
	writeAuthTokenResponse(w, token, int(a.config.Jwt.AuthTokenDuration.Seconds()), user)
}

// todo already verified.
// goroutine generates token
// RequestVerificationHandler handles email verification requests
// Endpoint: POST /request-verification
func (a *App) RequestVerificationHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, errorInvalidRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		writeJSONError(w, errorInvalidRequest)
		return
	}
	if err := ValidateEmail(req.Email); err != nil {
		writeJSONError(w, errorInvalidRequest)
		return
	}

	// Check if email exists in system
	user, err := a.db.GetUserByEmail(req.Email)
	if err != nil {
		writeJSONErrorf(w, http.StatusInternalServerError, `{"error":"Database error: %s"}`, err.Error())
		return
	}
	if user == nil {
		writeJSONError(w, errorNotFound)
		return
	}

	// Check if user is already verified
	if user.Verified {
		writeJSONOk(w, okAlreadyVerified)
		return
	}

	// Create queue job
	payload, _ := json.Marshal(queue.PayloadEmailVerification{Email: req.Email})
	job := queue.Job{
		JobType: queue.JobTypeEmailVerification,
		Payload: payload,
	}

	// Insert into job queue with deduplication
	err = a.db.InsertJob(job)
	if err != nil {
		if err == db.ErrConstraintUnique {
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

// confirm-
//
//	user created per email, requires validation of email, we have already emaila dn user id in table
//
// queue job creates token like this:
//
//	{
//	 "email": "lipo@goole.com",
//	 "exp": 1736630179,
//	 "id": "m648zm0q421yfc0",
//	 "type": "verification"
//	}
//
// with a new verification secret, create method in crypto
// with map claim in a good place with signing key email, passwordhash
// receives token
// parse unverified, should have all fiedls above reject if no
// validate all fields
// we can not write in the db yet
// we get user password from table. build signed key and try to verify the message if verified
// run VerifyEmail, document no race conditions, why
// get id, builds sig key with verification email secret
// jwt validate signed
// set verified
// key := (m.TokenKey() + m.Collection().VerificationToken.Secret)
// is a jwt
//
//	{
//	 "collectionId": "_pb_users_auth_",
//	 "email": "lipo@google.com",
//	 "exp": 1736630179,
//	 "id": "m648zm0q421yfc0",
//	 "type": "verification"
//	}
func (a *App) ConfirmVerificationHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Token string `json:"token"`
	}

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, errorInvalidRequest)
		return
	}

	// Parse unverified claims to discrd fast
	claims, err := crypto.ParseJwtUnverified(req.Token)
	if err != nil {
		writeJSONError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Validate all required claims exist and have correct values
	if err := crypto.ValidateVerificationClaims(claims); err != nil {
		writeJSONError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Get user from database to get password hash for signing key
	user, err := a.db.GetUserById(claims[crypto.ClaimUserID].(string))
	if err != nil || user == nil {
		writeJSONError(w, errorNotFound)
		return
	}

	// Verify token signature using verification email secret
	signingKey, err := crypto.NewJwtSigningKeyWithCredentials(
		claims[crypto.ClaimEmail].(string),
		user.Password,
		a.config.Jwt.VerificationEmailSecret,
	)
	if err != nil {
		writeJSONError(w, errorEmailVerificationFailed)
		return
	}

	// Fully verify token signature and claims
	_, err = crypto.ParseJwt(req.Token, signingKey)
	if err != nil {
		writeJSONError(w, errorJwtInvalidVerificationToken)
		return
	}

	// Check if user is already verified
	if user.Verified {
		writeJSONOk(w, okAlreadyVerified)
		return
	}

	// Mark user as verified
	err = a.db.VerifyEmail(user.ID)
	if err != nil {
		writeJSONError(w, errorServiceUnavailable)
		return
	}

	writeJSONOk(w, okEmailVerified)
}

// RegisterWithPasswordHandler handles password-based user registration with validation
// Endpoint: POST /register-with-password
// TODO we allow register with password after the user has oauth, we just
// update the password and do not require validated email as we trust the oauth2
// provider
// if password exist CreateUserWithPassword will succeed but the password will be not updated.
func (a *App) RegisterWithPasswordHandler(w http.ResponseWriter, r *http.Request) {

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

	// Validate password complexity TODO
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

	// Prepare user data
	newUser := db.User{
		Email:           req.Identity,
		Password:        string(hashedPassword),
		Name:            "", // Optional field TODO
		Verified:        false,
		Oauth2:          false,
		EmailVisibility: false,
	}

	// Create user with password authentication
	retrievedUser, err := a.db.CreateUserWithPassword(newUser)
	if err != nil {
		writeJSONErrorf(w, http.StatusInternalServerError, `{"error":"Registration failed: %s"}`, err.Error())
		return
	}

	// If passwords are different CreateUserWithPassword did not write the new
	// password on conflict because the user had already a password.
	if retrievedUser.Password != newUser.Password {
		writeJSONError(w, errorEmailConflict)
		return
	}

	// If user is not verified, add verification job to queue
	if !retrievedUser.Verified {
		payload, _ := json.Marshal(queue.PayloadEmailVerification{Email: retrievedUser.Email})
		job := queue.Job{
			JobType: queue.JobTypeEmailVerification,
			Payload: payload,
		}

		err = a.db.InsertJob(job)
		if err != nil {
			slog.Error("Failed to insert verification job", "error", err, "job", job)
			writeJSONError(w, errorServiceUnavailable)
			return
		}
	}

	// Generate JWT session token for immediate authentication
	token, _, err := crypto.NewJwtSessionToken(retrievedUser.ID, retrievedUser.Email, retrievedUser.Password, a.config.Jwt.AuthSecret, a.config.Jwt.AuthTokenDuration)
	if err != nil {
		writeJSONError(w, errorTokenGeneration)
		return
	}

	// Return standardized authentication token response
	writeAuthTokenResponse(w, token, int(a.config.Jwt.AuthTokenDuration.Seconds()), retrievedUser)
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
